package licensekey

import "crypto/ed25519"

// DefaultPublicKeyB64 adalah public key vendor yang DITANAM di binary server.
// Dengan ini server klien tidak perlu mengisi LICENSE_PUBLIC_KEY di .env.
//
// Untuk white-label / rotasi kunci, nilai ini bisa ditimpa saat build:
//
//	go build -ldflags "-X byone-arena/pkg/licensekey.DefaultPublicKeyB64=<base64>" ./cmd/server
//
// Private key tetap HANYA di komputer vendor (tool licensegen).
var DefaultPublicKeyB64 = "2PDipNENYanK2wAi0PRHG1KCRcoxM72RbXEUuKi_hoA"

// DefaultPublicKey mengembalikan public key bawaan dalam bentuk siap pakai.
func DefaultPublicKey() (ed25519.PublicKey, error) {
	return ParsePublicKey(DefaultPublicKeyB64)
}
