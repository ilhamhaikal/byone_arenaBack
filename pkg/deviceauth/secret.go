// Package deviceauth mengelola kredensial perangkat TV (secret acak yang
// dibuat di perangkat). Server hanya menyimpan hash SHA-256, bukan secret asli.
package deviceauth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"
)

// HashSecret menghasilkan hash SHA-256 dalam bentuk hex lowercase.
func HashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// IsValidHash memastikan hash berbentuk 64 karakter hex (SHA-256).
func IsValidHash(hash string) bool {
	hash = strings.TrimSpace(hash)
	if len(hash) != 64 {
		return false
	}
	_, err := hex.DecodeString(hash)
	return err == nil
}

// EqualHash membandingkan dua hash secara constant-time.
func EqualHash(a, b string) bool {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
