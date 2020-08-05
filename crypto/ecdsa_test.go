package cryptogo

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestGenerateKeyPairs(t *testing.T) {
	pri, pub, err := GenerateKeyPairs()
	t.Logf("%s %s %v", pri, pub, err)
	prikey, err := LoadPrivateKey(pri)
	if err != nil {
		t.Error(err)
	}
	pubkey, err := LoadPublicKey(pub)
	if err != nil {
		t.Error(err)
	}

	txt := "blockchain Is good"
	hash := sha256.New().Sum([]byte(txt))

	signCentent, err := Sign(prikey, hash)
	if err != nil {
		t.Error(err)
	}
	t.Logf("签名结果为: %s", signCentent)

	if !VerifySign(pubkey, signCentent, hex.EncodeToString(hash)) {
		t.Fatalf("验证签名失败")
	}
}

func TestLoadPriPubKey(t *testing.T) {
	pri := "0xf25ccbf8a1bb36594d5f63e9564ca4c5d965ccf8b418e8717f2f68b600cf6a34"
	pub := "0xc4024ffd0b42495f49002b5da606512aee341c53e43a641b7d8efac8e29f6ed2d5c6449fe4343f41c5216a84ea9dd43e07daeeadb38556bb19527ce699394cd7"
	priKey, err := LoadPrivateKey(pri)
	if err != nil {
		t.Error(err)
	}
	pubKey, err := LoadPublicKey(pub)
	if priKey.PublicKey.X.String() != pubKey.X.String() {
		t.Fatalf("公钥不一致")
	}
	if priKey.PublicKey.Y.String() != pubKey.Y.String() {
		t.Fatalf("公钥不一致")
	}

}

func TestSign(t *testing.T) {
	pri := "0xf25ccbf8a1bb36594d5f63e9564ca4c5d965ccf8b418e8717f2f68b600cf6a34"
	pub := "0xc4024ffd0b42495f49002b5da606512aee341c53e43a641b7d8efac8e29f6ed2d5c6449fe4343f41c5216a84ea9dd43e07daeeadb38556bb19527ce699394cd7"
	priKey, err := LoadPrivateKey(pri)
	if err != nil {
		t.Error(err)
	}
	pubKey, err := LoadPublicKey(pub)
	if priKey.PublicKey.X.String() != pubKey.X.String() {
		t.Fatalf("公钥不一致")
	}
	if priKey.PublicKey.Y.String() != pubKey.Y.String() {
		t.Fatalf("公钥不一致")
	}

	txt := "blockchain Is good"
	hash := sha256.New().Sum([]byte(txt))

	signCentent, err := Sign(priKey, hash)
	if err != nil {
		t.Error(err)
	}
	t.Logf("签名结果为: %s", signCentent)

	if !VerifySign(pubKey, signCentent, hex.EncodeToString(hash)) {
		t.Fatalf("验证签名失败")
	}

}
