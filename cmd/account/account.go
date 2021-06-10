package account

import (
	"fmt"
	"io"
	"os"
	"strings"

	cryptogo "github.com/wupeaking/pbft_impl/crypto"
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
