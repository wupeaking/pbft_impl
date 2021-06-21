package model

import (
	"crypto/sha256"
	"fmt"
	"math/big"

	"github.com/golang/protobuf/proto"
	"github.com/wupeaking/pbft_impl/common"
	cryptogo "github.com/wupeaking/pbft_impl/crypto"
)

func PublicKeyToAddress(pub []byte) *Address {
	hash := sha256.New().Sum(pub)
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
	}
	b, err := proto.Marshal(t)
	if err != nil {
		return false, err
	}
	sh := sha256.New()
	sh.Write(b)
	hash := sh.Sum(nil)
	pub, err := cryptogo.LoadPublicKeyFromBytes(tx.PublickKey)
	return cryptogo.VerifySign(pub, fmt.Sprintf("%0x", tx.Sign), fmt.Sprintf("0x%x", hash)), nil
}

func (txs *Txs) MerkleRoot() []byte {
	txSigns := make([][]byte, len(txs.Tansactions))
	for i := range txs.Tansactions {
		txSigns = append(txSigns, txs.Tansactions[i].Sign)
	}
	return common.Merkel(txSigns)
}

func (txRs *TxReceipts) MerkleRoot() []byte {
	txrSigns := make([][]byte, len(txRs.TansactionReceipts))
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
