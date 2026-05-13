package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

func deriveAPIKeyAESKey(appSecret string) []byte {
	sum := sha256.Sum256([]byte(appSecret + "|ai-gateway-user-api-key-v1"))
	return sum[:]
}

// EncryptAPIKeySecret stores the raw API key for owner retrieval (AES-256-GCM).
func EncryptAPIKeySecret(appSecret, plaintext string) (string, error) {
	if appSecret == "" {
		return "", fmt.Errorf("missing app secret")
	}
	key := deriveAPIKeyAESKey(appSecret)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	out := append(append([]byte{}, nonce...), sealed...)
	return base64.StdEncoding.EncodeToString(out), nil
}

// DecryptAPIKeySecret reverses EncryptAPIKeySecret.
func DecryptAPIKeySecret(appSecret, encoded string) (string, error) {
	if encoded == "" {
		return "", fmt.Errorf("empty payload")
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	key := deriveAPIKeyAESKey(appSecret)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	ns := gcm.NonceSize()
	if len(raw) < ns {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := raw[:ns], raw[ns:]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
