package transaction

import "math/big"

type Address []byte
type Hash []byte
type Sign []byte

type Tx struct {
	Sender    Address
	Recipient Address
	Amount    big.Int
	Sequeue   string
	Input     []byte
}
