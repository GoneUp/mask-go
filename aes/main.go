package main

import (
	"crypto/aes"
	"encoding/hex"
	"fmt"
	"os"
)

//mini tool do decrypt shining mask aes-ecb
func main() {
	if len(os.Args) <= 1 {
		fmt.Println("usage: ./aes [hexstring, length must be divisable by 16]")
		os.Exit(1)
	}

	fmt.Println("input: ", os.Args[1])
	bytes := DecryptAes128Ecb(os.Args[1])
	s := hex.EncodeToString(bytes)
	fmt.Println("Decypted (hex): ", s)
	fmt.Println("Decypted (unicode): ", string(bytes))
}

func DecryptAes128Ecb(hexstring string) []byte {
	data, err := hex.DecodeString(hexstring)
	must("data", err)

	key, err := hex.DecodeString("32672f7974ad43451d9c6c894a0e8764")
	must("key", err)
	cipher, err := aes.NewCipher([]byte(key))
	must("cipher", err)

	decrypted := make([]byte, len(data))
	size := 16

	for bs, be := 0, size; bs < len(data); bs, be = bs+size, be+size {
		cipher.Decrypt(decrypted[bs:be], data[bs:be])
	}

	return decrypted
}

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}
