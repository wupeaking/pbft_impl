package common

import (
	"crypto/sha256"
	"os"
)

func Merkel(arrs [][]byte) []byte {
	if len(arrs) == 0 {
		sh := sha256.New()
		return sh.Sum(nil)
	}
	if len(arrs) == 1 {
		sh := sha256.New()
		sh.Write(arrs[0])
		return sh.Sum(nil)
	}
	if len(arrs) == 2 {
		sh := sha256.New()
		sh.Write(arrs[0])
		sh.Write(arrs[1])
		return sh.Sum(nil)
	}
	newArrs := make([][]byte, 0)
	i := 0
	for {
		if i+1 >= len(arrs) {
			break
		}
		newArrs = append(newArrs, Merkel([][]byte{arrs[i], arrs[i+1]}))
		i += 2
	}
	if i == len(arrs)-1 {
		newArrs = append(newArrs, Merkel([][]byte{arrs[i]}))
	}
	return Merkel(newArrs)
}

func FileExist(file string) bool {
	_, err := os.Stat(file) //os.Stat获取文件信息
	if err != nil {
		return os.IsExist(err)
	}
	return true
}
