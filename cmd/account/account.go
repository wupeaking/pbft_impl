package account

import (
	"fmt"
	"io"
	"os"
	"strings"

	cryptogo "github.com/wupeaking/pbft_impl/crypto"
	"github.com/wupeaking/pbft_impl/model"
)

func GenerateAccount(file string, visibility bool) error {
	pri, pub, err := cryptogo.GenerateKeyPairs()
	if err != nil {
		return err
	}

	if visibility {
		fmt.Printf(`
		private_key: %s
		public_key: %s
		
		`, pri, pub)
	}
	if file == "" {
		return nil
	}
	f, err := os.Create(file)
	if err != nil {
		return err
	}

	_, err = io.Copy(f, strings.NewReader(fmt.Sprintf(`
	{
		"private": %s,
		"public": %s
	}
	`, pri, pub)))
	return err
}

func PublicKeyToAddress(pub string) error {
	pubByte, err := cryptogo.Hex2Bytes(pub)
	if err != nil {
		return err
	}
	println(len(pubByte))

	addr := model.PublicKeyToAddress(pubByte)
	fmt.Printf("addres: %s \n", addr.Address)
	return err
}
