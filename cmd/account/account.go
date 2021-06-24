package account

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	uuid "github.com/satori/go.uuid"
	cryptogo "github.com/wupeaking/pbft_impl/crypto"
	"github.com/wupeaking/pbft_impl/model"
)

func GenerateAccount(password string) error {
	pri, pub, err := cryptogo.GenerateKeyPairs()
	if err != nil {
		return err
	}

	db, err := sqlx.Open("sqlite3", "account.db")
	defer db.Close()
	if err != nil {
		return err
	}
	var schema = `
	CREATE TABLE if not exists account_info (
		id    integer PRIMARY KEY autoincrement,
		address VARCHAR(256)  DEFAULT '',
		public  VARCHAR(256)  DEFAULT '',
		private VARCHAR(256) DEFAULT ''
	);
`
	_, err = db.Exec(schema)
	if err != nil {
		return err
	}
	priBytes, _ := cryptogo.Hex2Bytes(pri)
	priCrypto := cryptogo.AESEncrypt(priBytes, []byte(password))
	priCryptoStr := cryptogo.Bytes2Hex(priCrypto)
	pubByte, err := cryptogo.Hex2Bytes(pub)
	if err != nil {
		return err
	}

	smt := fmt.Sprintf("insert into account_info(address, public, private) values ($1, $2, $3)")
	_, err = db.Exec(smt, model.PublicKeyToAddress(pubByte).Address, pub, priCryptoStr)
	if err != nil {
		return err
	}

	fmt.Printf(`
		private_key: %s
		public_key: %s
		address: %s
		`, pri, pub, model.PublicKeyToAddress(pubByte).Address)
	return err
}

func PublicKeyToAddress(pub string) error {
	pubByte, err := cryptogo.Hex2Bytes(pub)
	if err != nil {
		return err
	}
	addr := model.PublicKeyToAddress(pubByte)
	fmt.Printf("addres: %s \n", addr.Address)
	return err
}

func Balance(api string) error {
	db, err := sqlx.Open("sqlite3", "account.db")
	defer db.Close()
	if err != nil {
		return err
	}
	rows, err := db.Queryx("select address from account_info")
	if err != nil {
		return err
	}

	balance := uint64(0)
	defer rows.Close()
	num := 0
	for rows.Next() {
		num++
		var address string
		if err := rows.Scan(&address); err != nil {
			return err
		}
		resp, err := http.Get(api + "/account/" + address)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		content, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		respValue := struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			// Data    *model.Account `json:"data"`
		}{}
		if err := json.Unmarshal(content, &respValue); err != nil {
			return err
		}
		if respValue.Code == 404 {
			continue
		}
		if respValue.Code == 0 {
			respValue := struct {
				Code    int            `json:"code"`
				Message string         `json:"message"`
				Data    *model.Account `json:"data"`
			}{}
			if err := json.Unmarshal(content, &respValue); err != nil {
				return err
			}
			b, _ := strconv.ParseUint(respValue.Data.Balance.Amount, 10, 64)
			balance += b
			continue
		}
		return fmt.Errorf(respValue.Message)
	}

	fmt.Printf("账户数量: %d 余额: %d\n", num, balance)
	return nil
}

func Transfer(api, to, password, address string, index int, amount int64) error {
	if address == "" && index == -1 {
		return fmt.Errorf("账户地址或者编号必须任选其一")
	}
	db, err := sqlx.Open("sqlite3", "account.db")
	defer db.Close()
	if err != nil {
		return err
	}
	var pub, pri string
	if address != "" {
		row := db.QueryRowx("select public, private from account_info where address = $1", address)
		err := row.Scan(&pub, &pri)
		if err == sql.ErrNoRows {
			return fmt.Errorf("此地址的账户不存在")
		}
		if err != nil {
			return err
		}
	} else {
		row := db.QueryRowx("select public, private, address from account_info where id = $1", index)
		err := row.Scan(&pub, &pri, &address)
		if err == sql.ErrNoRows {
			return fmt.Errorf("此序号的账户不存在")
		}
		if err != nil {
			return err
		}
	}

	priCrypro, _ := cryptogo.Hex2Bytes(pri)
	// 解密私钥
	private, err := cryptogo.AESDecrypt(priCrypro, []byte(password))
	if err != nil {
		return err
	}
	privateKey, err := cryptogo.LoadPrivateKey(cryptogo.Bytes2Hex(private))
	if err != nil {
		return err
	}
	tx := &model.Tx{
		Sender:    &model.Address{Address: address},
		Recipient: &model.Address{Address: to},
		Sequeue:   strings.Replace(uuid.NewV4().String(), "-", "", -1),
		TimeStamp: uint64(time.Now().Unix()),
		Amount:    &model.Amount{Amount: fmt.Sprintf("%d", amount)},
	}
	err = tx.SignTx(privateKey)
	if err != nil {
		return err
	}

	url := api + "/tx/tansaction/" + cryptogo.Bytes2Hex(tx.Sign)
	request := struct {
		From      string `json:"from"`
		To        string `json:"to"`
		Amount    uint64 `json:"amount"`
		Sign      string `json:"sign"`
		PublicKey string `json:"publick_key"`
		Sequeue   string `json:"sequeue"`
		Timestamp uint64 `json:"timestamp"`
	}{From: address, To: to, Amount: uint64(amount),
		Sign:      cryptogo.Bytes2Hex(tx.Sign),
		PublicKey: cryptogo.Bytes2Hex(tx.PublickKey),
		Sequeue:   tx.Sequeue,
		Timestamp: tx.TimeStamp,
	}
	fmt.Printf("request: %#v\n", request)
	reqBody, err := json.Marshal(request)
	if err != nil {
		return err
	}

	payload := bytes.NewReader(reqBody)
	req, _ := http.NewRequest("PUT", url, payload)
	req.Header.Add("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	fmt.Printf("%v\n", string(body))
	return nil
}

func List(password string) error {
	db, err := sqlx.Open("sqlite3", "account.db")
	defer db.Close()
	if err != nil {
		return err
	}
	rows, err := db.Queryx("select address, public, private, id from account_info")
	if err != nil {
		return err
	}

	defer rows.Close()
	num := 0
	fmt.Printf("-------------------------\n")
	for rows.Next() {
		num++
		var address, pub, pri string
		var id int
		if err := rows.Scan(&address, &pub, &pri, &id); err != nil {
			return err
		}
		if password != "" {
			println(pri)
			priCrypro, err := cryptogo.Hex2Bytes(pri)
			if err != nil {
				return err
			}
			// 解密私钥
			// println("解密使用的密码为:", password)
			private, err := cryptogo.AESDecrypt(priCrypro, []byte(password))
			if err != nil {
				return err
			}
			pri = cryptogo.Bytes2Hex(private)
		}
		fmt.Printf("	address: %s, public: %s, private: %s, index: %d\n",
			address, pub, pri, id)
	}

	fmt.Printf("累计: %d \n", num)
	fmt.Printf("-------------------------\n")
	return nil
}

func Import(private, password string) error {
	pri, err := cryptogo.LoadPrivateKey(private)
	if err != nil {
		return err
	}
	pubStr := fmt.Sprintf("0x%x%x", pri.PublicKey.X.Bytes(),
		pri.PublicKey.Y.Bytes())
	priStr := fmt.Sprintf("0x%x", pri.D.Bytes())
	pubBytes, _ := cryptogo.Hex2Bytes(pubStr)
	address := model.PublicKeyToAddress(pubBytes)

	// 对私钥进行加密
	priBytes, _ := cryptogo.Hex2Bytes(priStr)
	println("加密的密码为: %s", password)
	priCrypto := cryptogo.AESEncrypt(priBytes, []byte(password))
	priCryptoStr := cryptogo.Bytes2Hex(priCrypto)

	db, err := sqlx.Open("sqlite3", "account.db")
	defer db.Close()
	if err != nil {
		return err
	}
	ret, err := db.Exec("insert into account_info(address, public, private) values($1, $2, $3)", address.Address, pubStr, priCryptoStr)
	if err != nil {
		return err
	}
	id, _ := ret.LastInsertId()
	fmt.Printf("导入成功 序号: %d \n", id)
	return nil
}
