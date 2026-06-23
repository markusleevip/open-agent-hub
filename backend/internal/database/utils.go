package database

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"strconv"

	"golang.org/x/crypto/bcrypt"
)

var encryptionKey []byte

func SetEncryptionKey(key string) {
	k := []byte(key)
	if len(k) < 32 {
		padded := make([]byte, 32)
		copy(padded, k)
		k = padded
	}
	encryptionKey = k[:32]
}

func EncryptAES(plaintext string) string {
	if encryptionKey == nil || plaintext == "" {
		return plaintext
	}
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return plaintext
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return plaintext
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return plaintext
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed)
}

func DecryptAES(ciphertext string) string {
	if encryptionKey == nil || ciphertext == "" {
		return ciphertext
	}
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return ciphertext
	}
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return ciphertext
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return ciphertext
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return ciphertext
	}
	nonce, sealed := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return ciphertext
	}
	return string(plaintext)
}

// randomString generates a random string of the given byte length
func randomString(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func bcryptHashForToken(token string) string {
	hash, _ := bcrypt.GenerateFromPassword([]byte(token), 10)
	return string(hash)
}

func stringArrayToJSON(arr []string) string {
	b, _ := json.Marshal(arr)
	return string(b)
}

func mustAtoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
