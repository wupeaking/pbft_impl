package model

import (
	"crypto/sha256"
	"fmt"
)

func PublicKeyToAddress(pub []byte) *Address {
	hash := sha256.New().Sum(pub)
	hexStr := fmt.Sprintf("0x%x", hash)
	return &Address{Address: hexStr}
}
