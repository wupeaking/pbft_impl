package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	uuid "github.com/satori/go.uuid"
	"github.com/wupeaking/pbft_impl/common"
	cryptogo "github.com/wupeaking/pbft_impl/crypto"
	"github.com/wupeaking/pbft_impl/model"
)

type Account struct {
	Pri  string `json:"pri"`
	Pub  string `json:"pub"`
	Addr string `json:"addr"`
}

var api = "http://localhost:8088"

func main() {
	accs := make([]Account, 0, 100)
	fileExist := common.FileExist("accounts.json")
	if fileExist {
		f, err := os.Open("accounts.json")
		if err != nil {
			panic(err)
		}
		content, _ := ioutil.ReadAll(f)
		if err := json.Unmarshal(content, &accs); err != nil {
			panic(err)
		}
		f.Close()
	} else {
		// 生成100个账户
		for i := 0; i < 100; i++ {
			pri, pub, err := cryptogo.GenerateKeyPairs()
			if err != nil {
				panic(err)
			}
			pubByte, _ := cryptogo.Hex2Bytes(pub)

			accs = append(accs, Account{pri, pub, model.PublicKeyToAddress(pubByte).Address})
		}
		f, err := os.Create("accounts.json")
		if err != nil {
			panic(err)
		}
		v, _ := json.Marshal(accs)
		io.Copy(f, strings.NewReader(string(v)))
		f.Close()
	}

	privateKey, err := cryptogo.LoadPrivateKey("0xf25ccbf8a1bb36594d5f63e9564ca4c5d965ccf8b418e8717f2f68b600cf6a34")
	if err != nil {
		panic(err)
	}
	// 往每个账户转1000
	for i := 0; i < 100 && !fileExist; i++ {
		tx := &model.Tx{
			Sender:    &model.Address{Address: "0xf52772d71e21a42e8cd2c5987ed3bb99420fecf4c7aca797b926a8f01ea6ffd8"},
			Recipient: &model.Address{Address: accs[i].Addr},
			Sequeue:   strings.Replace(uuid.NewV4().String(), "-", "", -1),
			TimeStamp: uint64(time.Now().Unix()),
			Amount:    &model.Amount{Amount: fmt.Sprintf("%d", 100)},
		}
		err = tx.SignTx(privateKey)
		if err != nil {
			panic(err)
		}

		url := api + "/tx/transaction/" + cryptogo.Bytes2Hex(tx.Sign)
		request := struct {
			From      string `json:"from"`
			To        string `json:"to"`
			Amount    uint64 `json:"amount"`
			Sign      string `json:"sign"`
			PublicKey string `json:"publick_key"`
			Sequeue   string `json:"sequeue"`
			Timestamp uint64 `json:"timestamp"`
		}{From: tx.Sender.Address, To: tx.Recipient.Address, Amount: uint64(100),
			Sign:      cryptogo.Bytes2Hex(tx.Sign),
			PublicKey: cryptogo.Bytes2Hex(tx.PublickKey),
			Sequeue:   tx.Sequeue,
			Timestamp: tx.TimeStamp,
		}
		// fmt.Printf("request: %#v\n", request)
		reqBody, err := json.Marshal(request)
		if err != nil {
			panic(err)
		}

		payload := bytes.NewReader(reqBody)
		req, _ := http.NewRequest("PUT", url, payload)
		req.Header.Add("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}
		fmt.Printf("%v\n", string(body))
		time.Sleep(500 * time.Millisecond)
	}

	for {
		// 每隔800毫秒 随机转账
		time.Sleep(800 * time.Millisecond)
		f := rand.Intn(100)
		t := rand.Intn(100)
		privateKey, err = cryptogo.LoadPrivateKey(accs[f].Pri)
		if err != nil {
			fmt.Println("加载私钥错误: err:", err)
			continue
		}
		amount := rand.Intn(5)
		if amount == 0 {
			amount = 1
		}
		tx := &model.Tx{
			Sender:    &model.Address{Address: accs[f].Addr},
			Recipient: &model.Address{Address: accs[t].Addr},
			Sequeue:   strings.Replace(uuid.NewV4().String(), "-", "", -1),
			TimeStamp: uint64(time.Now().Unix()),
			Amount:    &model.Amount{Amount: fmt.Sprintf("%d", amount)},
		}
		err = tx.SignTx(privateKey)
		if err != nil {
			panic(err)
		}

		url := api + "/tx/transaction/" + cryptogo.Bytes2Hex(tx.Sign)
		request := struct {
			From      string `json:"from"`
			To        string `json:"to"`
			Amount    uint64 `json:"amount"`
			Sign      string `json:"sign"`
			PublicKey string `json:"publick_key"`
			Sequeue   string `json:"sequeue"`
			Timestamp uint64 `json:"timestamp"`
		}{From: tx.Sender.Address, To: tx.Recipient.Address, Amount: uint64(amount),
			Sign:      cryptogo.Bytes2Hex(tx.Sign),
			PublicKey: cryptogo.Bytes2Hex(tx.PublickKey),
			Sequeue:   tx.Sequeue,
			Timestamp: tx.TimeStamp,
		}
		reqBody, err := json.Marshal(request)
		if err != nil {
			fmt.Println("序列化失败: err:", err)
			continue
		}

		payload := bytes.NewReader(reqBody)
		req, _ := http.NewRequest("PUT", url, payload)
		req.Header.Add("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Println("请求URL失败: err:", err)
			continue
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("读取response失败: err:", err)
			continue
		}
		fmt.Printf("%v\n", string(body))
		fmt.Printf("from: %s, to: %s, amount: %s\n", tx.Sender.Address, tx.Recipient.Address, tx.Amount.Amount)
	}
}
