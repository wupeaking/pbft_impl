package account

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	cryptogo "github.com/wupeaking/pbft_impl/crypto"
	"github.com/wupeaking/pbft_impl/model"
)

func GenerateAccount(password string) error {
	pri, pub, err := cryptogo.GenerateKeyPairs()
	if err != nil {
		return err
	}

	db, err := sqlx.Open("sqlite3", "account.db")
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
	priCrypto, err := cryptogo.AesEncrypt(priBytes, []byte(password))
	if err != nil {
		return err
	}
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
