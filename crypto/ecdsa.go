package cryptogo

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

// private publick error
func GenerateKeyPairs() (string, string, error) {
	r, err := random(48)
	if err != nil {
		return "", "", err
	}
	pri, err := ecdsa.GenerateKey(elliptic.P256(), bytes.NewBuffer(r))
	if err != nil {
		return "", "", err
	}

	return fmt.Sprintf("0x%x", pri.D.Bytes()),
		fmt.Sprintf("0x%x%x", pri.PublicKey.X.Bytes(),
			pri.PublicKey.Y.Bytes()),
		nil
}

func LoadPrivateKey(pri string) (*ecdsa.PrivateKey, error) {
	d := new(big.Int)
	_, ok := d.SetString(pri, 0)
	if !ok {
		return nil, fmt.Errorf("私钥格式错误")
	}
	priv := new(ecdsa.PrivateKey)
	priv.PublicKey.Curve = elliptic.P256()
	priv.D = d
	priv.PublicKey.X, priv.PublicKey.Y = elliptic.P256().ScalarBaseMult(d.Bytes())
	return priv, nil
}

func LoadPublicKey(public string) (*ecdsa.PublicKey, error) {
	if strings.HasPrefix(public, "0x") || strings.HasPrefix(public, "0X") {
		public = public[2:]
	}
	xy, err := hex.DecodeString(public)
	if err != nil {
		return nil, err
	}
	if len(xy) != 64 {
		return nil, fmt.Errorf("公钥格式错误")
	}
	x := xy[0:32]
	y := xy[32:]

	publicKey := ecdsa.PublicKey{Curve: elliptic.P256()}
	publicKey.X = new(big.Int).SetBytes(x)
	publicKey.Y = new(big.Int).SetBytes(y)
	return &publicKey, nil
}

func random(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	return b, err
}

func Sign(priv *ecdsa.PrivateKey, conetnt []byte) (string, error) {
	rd, err := random(32)
	if err != nil {
		return "", err
	}
	r, s, err := ecdsa.Sign(bytes.NewBuffer(rd), priv, conetnt)
	if err != nil {
		return "", err
	}

	// if len(r.Bytes()) != 32 || len(s.Bytes()) != 32 {
	// 	panic(fmt.Sprintf("%s %s", r.Text(16), s.Text(16)))
	// }

	return fmt.Sprintf("0x%064x%064x", r.Bytes(), s.Bytes()), nil
}

// VerifySign hash为16进制格式
func VerifySign(pub *ecdsa.PublicKey, sign string, hash string) bool {
	var hashBytes []byte
	if strings.HasPrefix(hash, "0x") || strings.HasPrefix(hash, "0X") {
		hash = hash[2:]
	}
	hashBytes, err := hex.DecodeString(hash)
	if err != nil {
		return false
	}

	var signBytes []byte
	if strings.HasPrefix(sign, "0x") || strings.HasPrefix(sign, "0X") {
		sign = sign[2:]
	}
	signBytes, err = hex.DecodeString(sign)
	if err != nil {
		return false
	}

	if len(signBytes) != 64 {
		return false
	}
	r := signBytes[0:32]
	s := signBytes[32:]

	return ecdsa.Verify(pub, hashBytes, new(big.Int).SetBytes(r), new(big.Int).SetBytes(s))
}

func Hex2Bytes(hexStr string) ([]byte, error) {
	if strings.HasPrefix(hexStr, "0x") || strings.HasPrefix(hexStr, "0X") {
		hexStr = hexStr[2:]
	}
	return hex.DecodeString(hexStr)
}
