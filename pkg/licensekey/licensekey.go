// Package licensekey mendefinisikan format lisensi offline yang ditandatangani
// vendor (Ed25519) beserta fungsi verifikasinya.
//
// Format teks lisensi:
//
//	base64url(payloadJSON) + "." + base64url(signature)
//
// Server hanya memegang public key, sehingga tidak bisa menerbitkan lisensi
// sendiri. Perubahan pada payload (mis. tanggal kedaluwarsa) merusak tanda
// tangan dan ditolak.
package licensekey

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Payload adalah isi lisensi yang ditandatangani vendor.
type Payload struct {
	Version   int    `json:"v"`
	LicenseID string `json:"licenseId"`
	Customer  string `json:"customer"`
	Plan      string `json:"plan"`
	// MaxUnits = 0 berarti tanpa batas unit (enterprise).
	MaxUnits  int       `json:"maxUnits"`
	GraceDays int       `json:"graceDays,omitempty"`
	Features  []string  `json:"features,omitempty"`
	IssuedAt  time.Time `json:"issuedAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	// InstanceID mengikat lisensi ke satu server (UUID milik server klien).
	InstanceID      string `json:"instanceId"`
	ParentLicenseID string `json:"parentLicenseId,omitempty"`
}

// Unlimited true bila lisensi tidak membatasi jumlah unit.
func (p Payload) Unlimited() bool { return p.MaxUnits == 0 }

// Sign membuat teks lisensi bertanda tangan dari payload.
func Sign(p Payload, priv ed25519.PrivateKey) (string, error) {
	if len(priv) != ed25519.PrivateKeySize {
		return "", errors.New("private key tidak valid")
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	sig := ed25519.Sign(priv, raw)
	return base64.RawURLEncoding.EncodeToString(raw) + "." +
		base64.RawURLEncoding.EncodeToString(sig), nil
}

// Verify memeriksa tanda tangan lisensi lalu mengembalikan payload-nya.
func Verify(license string, pub ed25519.PublicKey) (*Payload, error) {
	license = strings.TrimSpace(license)
	if license == "" {
		return nil, errors.New("lisensi kosong")
	}
	parts := strings.Split(license, ".")
	if len(parts) != 2 {
		return nil, errors.New("format lisensi tidak valid")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errors.New("isi lisensi rusak")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("tanda tangan lisensi rusak")
	}
	if len(pub) != ed25519.PublicKeySize {
		return nil, errors.New("public key lisensi belum dikonfigurasi")
	}
	if !ed25519.Verify(pub, raw, sig) {
		return nil, errors.New("tanda tangan lisensi tidak valid (file rusak atau bukan dari vendor)")
	}

	var p Payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, errors.New("isi lisensi tidak dapat dibaca")
	}
	return &p, nil
}

// ValidateStructure memastikan payload masuk akal dan memang untuk server ini.
func ValidateStructure(p *Payload, instanceID uuid.UUID) error {
	if p == nil {
		return errors.New("payload lisensi kosong")
	}
	if p.Version != 1 {
		return fmt.Errorf("versi lisensi tidak didukung (%d)", p.Version)
	}
	if strings.TrimSpace(p.LicenseID) == "" {
		return errors.New("ID lisensi kosong")
	}
	if p.MaxUnits < 0 {
		return errors.New("jumlah unit lisensi tidak valid")
	}
	if p.GraceDays < 0 || p.GraceDays > 90 {
		return errors.New("masa tenggang lisensi tidak valid")
	}
	if p.ExpiresAt.IsZero() {
		return errors.New("tanggal kedaluwarsa lisensi kosong")
	}
	licInstance, err := uuid.Parse(strings.TrimSpace(p.InstanceID))
	if err != nil {
		return errors.New("ID server pada lisensi tidak valid")
	}
	if instanceID == uuid.Nil || licInstance != instanceID {
		return errors.New("lisensi ini diterbitkan untuk server lain")
	}
	return nil
}

// ParsePublicKey menerima public key Ed25519 dalam base64 (raw-url atau standar).
func ParsePublicKey(encoded string) (ed25519.PublicKey, error) {
	raw, err := decodeBase64(encoded)
	if err != nil {
		return nil, fmt.Errorf("public key tidak valid: %w", err)
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("panjang public key salah (%d byte)", len(raw))
	}
	return ed25519.PublicKey(raw), nil
}

// ParsePrivateKey menerima private key Ed25519 base64 (64 byte) atau seed (32 byte).
func ParsePrivateKey(encoded string) (ed25519.PrivateKey, error) {
	raw, err := decodeBase64(encoded)
	if err != nil {
		return nil, fmt.Errorf("private key tidak valid: %w", err)
	}
	switch len(raw) {
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(raw), nil
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(raw), nil
	default:
		return nil, fmt.Errorf("panjang private key salah (%d byte)", len(raw))
	}
}

func decodeBase64(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, errors.New("kosong")
	}
	if b, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return nil, errors.New("bukan base64 yang valid")
}
