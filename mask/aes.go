package mask

import (
	"crypto/aes"
	"encoding/hex"
)

// Takes hex string, len must be divsiable by 16
func EncryptAes128Hex(hexstring string) []byte {
	data, err := hex.DecodeString(hexstring)
	must("key", err)

	return EncryptAes128(data)
}

// Len must be divsiable by 16
// AES-ECB
func EncryptAes128(data []byte) []byte {
	//validate
	blockSize := 16
	if len(data) != blockSize {
		panic(0)
	}

	// create cipher
	key, err := hex.DecodeString("32672f7974ad43451d9c6c894a0e8764")
	must("key", err)
	c, err := aes.NewCipher(key)
	must("cipher", err)

	// allocate space for ciphered data
	out := make([]byte, blockSize)

	// encrypt
	c.Encrypt(out, data)

	return out
}

// Len must be divsiable by 16
// AES-ECB
func DecryptAes128Ecb(data []byte) []byte {
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


