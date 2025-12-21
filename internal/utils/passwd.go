package utils

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

func GenerateSalt(n uint32) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func HashPassword(password string) (string, error) {
	var (
		memory      uint32 = 64 * 1024 // 64 MB
		iterations  uint32 = 3
		parallelism uint8  = 2
		saltLength  uint32 = 16
		keyLength   uint32 = 32
	)

	salt, err := GenerateSalt(saltLength)
	if err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, keyLength)
	encoded := fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		memory, iterations, parallelism, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash))
	return encoded, nil
}

func VerifyPassword(encodedHash string, password string) (bool, error) {
	var salt, hash []byte
	var iterations, memory uint32
	var parallelism uint8
	var keyLength uint32

	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return false, fmt.Errorf("input hash is incorrect")
	}
	params := parts[3]
	_, err := fmt.Sscanf(params, "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism)
	if err != nil {
		return false, fmt.Errorf("input hash is incorrect")
	}
	saltEncoded := parts[4]
	salt, err = base64.RawStdEncoding.DecodeString(saltEncoded)
	if err != nil {
		return false, err
	}
	hashEncoded := parts[5]
	hash, err = base64.RawStdEncoding.DecodeString(hashEncoded)
	if err != nil {
		return false, err
	}
	keyLength = uint32(len(hash))
	newHash := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, keyLength)
	if bytes.Equal(newHash, hash) {
		return true, nil
	}
	return false, nil
}
