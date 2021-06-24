package cryptogo

import (
	"fmt"
	"testing"
)

func TestAes(t *testing.T) {
	source := "0xf25ccbf8a1bb36594d5f63e9564ca4c5d965ccf8b418e8717f2f68b600cf6a34"
	sourceBytes, _ := Hex2Bytes(source)
	fmt.Println("原字符：", source)
	//16byte密钥
	key := "123456"
	encryptCode := AESEncrypt(sourceBytes, []byte(key))
	fmt.Println("密文：", Bytes2Hex(encryptCode))

	decryptCode, _ := AESDecrypt(encryptCode, []byte("11111"))

	fmt.Println("解密：", string(decryptCode))
}
