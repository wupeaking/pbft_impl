package model

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"fmt"
	"math/big"

	"github.com/golang/protobuf/proto"
	"github.com/wupeaking/pbft_impl/common"
	cryptogo "github.com/wupeaking/pbft_impl/crypto"
)

func PublicKeyToAddress(pub []byte) *Address {
	h := sha256.New()
	h.Write(pub)
	hash := h.Sum(nil)
	hexStr := fmt.Sprintf("0x%x", hash)
	return &Address{Address: hexStr}
}

// Compare a==b: 0  a>b:1 a<b:-1
func Compare(a, b string) int {
	biga, _ := big.NewInt(0).SetString(a, 0)
	bigb, _ := big.NewInt(0).SetString(b, 0)
	return biga.Cmp(bigb)
}

func (tx *Tx) VerifySignedTx() (bool, error) {
	t := &Tx{
		Recipient: tx.Recipient,
		Amount:    tx.Amount,
		Sequeue:   tx.Sequeue,
		Input:     tx.Input,
		TimeStamp: tx.TimeStamp,
	}
	b, err := proto.Marshal(t)
	if err != nil {
		return false, err
	}
	sh := sha256.New()
	sh.Write(b)
	hash := sh.Sum(nil)
	pub, err := cryptogo.LoadPublicKeyFromBytes(tx.PublickKey)
	if err != nil {
		return false, err
	}
	return cryptogo.VerifySign(pub, fmt.Sprintf("%0x", tx.Sign), fmt.Sprintf("0x%x", hash)), nil
}

func (tx *Tx) SignTx(priv *ecdsa.PrivateKey) error {
	t := &Tx{
		Recipient: tx.Recipient,
		Amount:    tx.Amount,
		Sequeue:   tx.Sequeue,
		Input:     tx.Input,
		TimeStamp: tx.TimeStamp,
	}
	b, err := proto.Marshal(t)
	if err != nil {
		return err
	}
	sh := sha256.New()
	sh.Write(b)
	hash := sh.Sum(nil)

	signed, err := cryptogo.Sign(priv, hash)
	if err != nil {
		return err
	}
	tx.Sign, _ = cryptogo.Hex2Bytes(signed)

	tx.PublickKey = make([]byte, 0)
	tx.PublickKey = append(tx.PublickKey, priv.PublicKey.X.Bytes()...)
	tx.PublickKey = append(tx.PublickKey, priv.PublicKey.Y.Bytes()...)
	return nil
}

func (tx *Tx) IsVaildTx() bool {
	if tx.Sender == nil || tx.Sender.Address == "" ||
		tx.Sequeue == "" || len(tx.Sign) == 0 || len(tx.PublickKey) == 0 ||
		tx.Recipient == nil || tx.Recipient.Address == "" {
		// todo:: 后期 可以容许recipent为空
		return false
	}
	ok, _ := tx.VerifySignedTx()
	return ok
}

func (txs *Txs) MerkleRoot() []byte {
	txSigns := make([][]byte, 0, len(txs.Tansactions))
	for i := range txs.Tansactions {
		txSigns = append(txSigns, txs.Tansactions[i].Sign)
	}
	return common.Merkel(txSigns)
}

func (txRs *TxReceipts) MerkleRoot() []byte {
	txrSigns := make([][]byte, 0, len(txRs.TansactionReceipts))
	for i := range txRs.TansactionReceipts {
		txrSigns = append(txrSigns, txRs.TansactionReceipts[i].Sign)
	}
	return common.Merkel(txrSigns)
}

func (acc *Account) AddBalance(am *Amount) {
	acc.Balance.AddAmount(am)
}

func (acc *Account) SubBalance(am *Amount) {
	acc.Balance.SubAmount(am)
}

func (am *Amount) AddAmount(amb *Amount) {
	biga, _ := big.NewInt(0).SetString(am.Amount, 0)
	bigb, _ := big.NewInt(0).SetString(amb.Amount, 0)
	am.Amount = big.NewInt(0).Add(biga, bigb).String()
}

func (am *Amount) SubAmount(amb *Amount) {
	biga, _ := big.NewInt(0).SetString(am.Amount, 0)
	bigb, _ := big.NewInt(0).SetString(amb.Amount, 0)
	am.Amount = big.NewInt(0).Sub(biga, bigb).String()
}

func (txr *TxReceipt) SignedTxReceipt(priv *ecdsa.PrivateKey) error {
	t := &TxReceipt{
		Status: txr.Status,
		TxId:   txr.TxId,
	}
	b, err := proto.Marshal(t)
	if err != nil {
		return err
	}
	sh := sha256.New()
	sh.Write(b)
	hash := sh.Sum(nil)

	sign, err := cryptogo.Sign(priv, hash)
	if err != nil {
		return err
	}
	txr.Sign, err = cryptogo.Hex2Bytes(sign)
	return err
}

func (txr *TxReceipt) VerifySignedTxReciept(publicKey []byte) (bool, error) {
	t := &TxReceipt{
		Status: txr.Status,
		TxId:   txr.TxId,
	}
	b, err := proto.Marshal(t)
	if err != nil {
		return false, err
	}
	sh := sha256.New()
	sh.Write(b)
	hash := sh.Sum(nil)
	pub, err := cryptogo.LoadPublicKeyFromBytes(publicKey)
	return cryptogo.VerifySign(pub, fmt.Sprintf("%0x", txr.Sign), fmt.Sprintf("0x%x", hash)), nil
}

func (txr *TxReceipt) IsVaildTxR(publicKey []byte) bool {
	if len(txr.Sign) == 0 || len(txr.TxId) == 0 {
		return false
	}
	ok, _ := txr.VerifySignedTxReciept(publicKey)
	return ok
}
