// Package licensegen berisi operasi yang HANYA dipakai oleh tool vendor
// (cmd/licensegen): membuat pasangan kunci dan menandatangani lisensi.
// Paket ini tidak boleh dipakai oleh server klien.
package licensegen

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"byone-arena/pkg/licensekey"
)

// GenerateKeypair membuat pasangan kunci Ed25519 (base64 raw-url).
func GenerateKeypair() (publicKeyB64, privateKeyB64 string, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("gagal membuat kunci: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(pub),
		base64.RawURLEncoding.EncodeToString(priv), nil
}

// Issue menandatangani payload lisensi menjadi teks lisensi.
func Issue(p licensekey.Payload, privateKeyB64 string) (string, error) {
	priv, err := licensekey.ParsePrivateKey(privateKeyB64)
	if err != nil {
		return "", err
	}
	return licensekey.Sign(p, priv)
}
