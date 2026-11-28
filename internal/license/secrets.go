package license

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
)

// secretBox mengenkripsi secret aktivasi sebelum disimpan di database.
// Secret hanya berlaku selama request aktivasi (default 30 menit), tetapi
// mengenkripsinya mengurangi dampak bila backup database bocor.
type secretBox struct {
	key []byte
}

func newSecretBox(serverSecret string) *secretBox {
	sum := sha256.Sum256([]byte("byone-license-activation-v1:" + serverSecret))
	return &secretBox{key: sum[:]}
}

func (b *secretBox) seal(plain string) (string, error) {
	block, err := aes.NewCipher(b.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

func (b *secretBox) open(encoded string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", errors.New("secret aktivasi rusak")
	}
	block, err := aes.NewCipher(b.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("secret aktivasi tidak valid")
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", errors.New("secret aktivasi tidak dapat dibuka")
	}
	return string(plain), nil
}
