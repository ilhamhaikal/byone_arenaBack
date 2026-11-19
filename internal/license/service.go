// Package license mengelola status lisensi server: memuat lisensi aktif,
// memverifikasi tanda tangannya, menghitung unit terpakai, menangani aktivasi
// dan menyimpan riwayat. Semua akses database lewat stored procedure.
package license

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"byone-arena/pkg/licensekey"
	"byone-arena/pkg/spname"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// State lisensi yang dikirim ke frontend.
const (
	StateActive   = "active"   // aktif, sisa > 30 hari
	StateExpiring = "expiring" // aktif, sisa <= 30 hari
	StateGrace    = "grace"    // sudah lewat expiry, masih masa tenggang
	StateExpired  = "expired"  // lewat masa tenggang → read-only
	StateTrial    = "trial"    // belum ada lisensi, masih masa uji coba
	StateNone     = "none"     // belum ada lisensi & uji coba habis
	StateInvalid  = "invalid"  // lisensi ada tapi gagal verifikasi
)

// ErrNeedsConfirmation dikembalikan bila lisensi baru menurunkan masa berlaku
// atau jumlah unit dan admin belum menyetujuinya.
var ErrNeedsConfirmation = errors.New("perlu konfirmasi")

// Status adalah ringkasan lisensi untuk UI & penegakan.
type Status struct {
	State            string     `json:"state"`
	Plan             string     `json:"plan"`
	Customer         string     `json:"customer"`
	LicenseID        string     `json:"licenseId"`
	MaxUnits         int        `json:"maxUnits"` // 0 = unlimited
	UnitsUsed        int        `json:"unitsUsed"`
	ExpiresAt        *time.Time `json:"expiresAt,omitempty"`
	DaysLeft         int        `json:"daysLeft"`
	GraceDays        int        `json:"graceDays"`
	InstanceID       string     `json:"instanceId"`
	TrialDaysLeft    int        `json:"trialDaysLeft"`
	CanStartBusiness bool       `json:"canStartBusiness"`
	CanBindUnit      bool       `json:"canBindUnit"`
	ExpiringSoon     bool       `json:"expiringSoon"`
	Message          string     `json:"message"`
	Warnings         []string   `json:"warnings,omitempty"`
}

// HistoryRow satu baris riwayat lisensi.
type HistoryRow struct {
	ID          uuid.UUID  `json:"id"          gorm:"column:id"`
	LicenseID   string     `json:"licenseId"   gorm:"column:license_id"`
	Customer    string     `json:"customer"    gorm:"column:customer"`
	Plan        string     `json:"plan"        gorm:"column:plan"`
	MaxUnits    int        `json:"maxUnits"    gorm:"column:max_units"`
	IssuedAt    *time.Time `json:"issuedAt"    gorm:"column:issued_at"`
	ExpiresAt   time.Time  `json:"expiresAt"   gorm:"column:expires_at"`
	ActivatedAt time.Time  `json:"activatedAt" gorm:"column:activated_at"`
	IsActive    bool       `json:"isActive"    gorm:"column:is_active"`
}

type activeRow struct {
	ID          uuid.UUID  `gorm:"column:id"`
	LicenseKey  string     `gorm:"column:license_key"`
	LicenseID   string     `gorm:"column:license_id"`
	Customer    string     `gorm:"column:customer"`
	Plan        string     `gorm:"column:plan"`
	MaxUnits    int        `gorm:"column:max_units"`
	IssuedAt    *time.Time `gorm:"column:issued_at"`
	ExpiresAt   time.Time  `gorm:"column:expires_at"`
	InstanceID  uuid.UUID  `gorm:"column:instance_id"`
	ActivatedAt time.Time  `gorm:"column:activated_at"`
	IsActive    bool       `gorm:"column:is_active"`
	RawPayload  []byte     `gorm:"column:raw_payload"`
}

type identityRow struct {
	ID        uuid.UUID `gorm:"column:id"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

type unitsRow struct {
	Units int `gorm:"column:units"`
}

// Service mengelola lisensi. Aman dipakai bersamaan (cache internal).
type Service struct {
	db         *gorm.DB
	publicKey  ed25519.PublicKey
	trialDays  int
	trialUnits int
	graceDays  int
	ttl        time.Duration

	mu       sync.RWMutex
	cached   *Status
	cachedAt time.Time
}

// NewService membuat service lisensi.
func NewService(db *gorm.DB, pub ed25519.PublicKey, trialDays, trialUnits, graceDays int) *Service {
	if trialDays <= 0 {
		trialDays = 14
	}
	if trialUnits <= 0 {
		trialUnits = 2
	}
	if graceDays < 0 {
		graceDays = 14
	}
	return &Service{
		db:         db,
		publicKey:  pub,
		trialDays:  trialDays,
		trialUnits: trialUnits,
		graceDays:  graceDays,
		ttl:        60 * time.Second,
	}
}

func (s *Service) invalidate() {
	s.mu.Lock()
	s.cached = nil
	s.mu.Unlock()
}

// Status mengembalikan status lisensi (di-cache 60 detik).
func (s *Service) Status(ctx context.Context) *Status {
	s.mu.RLock()
	if s.cached != nil && time.Since(s.cachedAt) < s.ttl {
		cp := *s.cached
		s.mu.RUnlock()
		return &cp
	}
	s.mu.RUnlock()

	st := s.load(ctx)

	s.mu.Lock()
	s.cached = st
	s.cachedAt = time.Now()
	s.mu.Unlock()

	cp := *st
	return &cp
}

func (s *Service) identity(ctx context.Context) identityRow {
	var r identityRow
	if s.db == nil {
		return r
	}
	_ = s.db.WithContext(ctx).Raw(
		"SELECT * FROM " + spname.Ident("GetOrCreateServerIdentity") + "()").Scan(&r).Error
	return r
}

func (s *Service) unitsUsed(ctx context.Context) int {
	if s.db == nil {
		return 0
	}
	var r unitsRow
	_ = s.db.WithContext(ctx).Raw(
		"SELECT * FROM " + spname.Ident("CountLicensedUnits") + "()").Scan(&r).Error
	return r.Units
}

func (s *Service) load(ctx context.Context) *Status {
	st := &Status{State: StateNone, GraceDays: s.graceDays}

	if s.db == nil {
		st.State = StateInvalid
		st.Message = "Database tidak tersedia sehingga lisensi tidak dapat diperiksa."
		return st
	}

	ident := s.identity(ctx)
	st.InstanceID = ident.ID.String()
	st.UnitsUsed = s.unitsUsed(ctx)
	now := time.Now()

	var row activeRow
	if err := s.db.WithContext(ctx).Raw(
		"SELECT * FROM " + spname.Ident("GetActiveLicense") + "()").Scan(&row).Error; err != nil {
		st.State = StateInvalid
		st.Message = "Gagal membaca data lisensi."
		return st
	}

	// Belum ada lisensi → masa uji coba.
	if row.ID == uuid.Nil {
		if ident.CreatedAt.IsZero() {
			ident.CreatedAt = now
		}
		trialEnd := ident.CreatedAt.AddDate(0, 0, s.trialDays)
		st.Plan = "TRIAL"
		st.MaxUnits = s.trialUnits
		if now.Before(trialEnd) {
			left := int(math.Ceil(trialEnd.Sub(now).Hours() / 24))
			if left < 0 {
				left = 0
			}
			st.State = StateTrial
			st.TrialDaysLeft = left
			st.CanStartBusiness = true
			st.CanBindUnit = st.UnitsUsed < s.trialUnits
			st.Message = fmt.Sprintf("Masa uji coba: %d hari lagi (maksimal %d unit).", left, s.trialUnits)
		} else {
			st.State = StateNone
			st.Message = "Belum ada lisensi. Impor file lisensi untuk melanjutkan."
		}
		return st
	}

	// Ada lisensi → wajib lolos verifikasi tanda tangan.
	if len(s.publicKey) != ed25519.PublicKeySize {
		st.State = StateInvalid
		st.Message = "Public key lisensi belum dikonfigurasi pada server."
		return st
	}
	p, err := licensekey.Verify(row.LicenseKey, s.publicKey)
	if err != nil {
		st.State = StateInvalid
		st.Message = "Lisensi tidak valid: " + err.Error()
		return st
	}
	if err := licensekey.ValidateStructure(p, ident.ID); err != nil {
		st.State = StateInvalid
		st.Message = "Lisensi tidak valid: " + err.Error()
		return st
	}

	// Gunakan nilai dari payload bertanda tangan, BUKAN kolom DB, agar
	// perubahan pada database tidak bisa menaikkan unit/masa berlaku.
	st.LicenseID = p.LicenseID
	st.Customer = p.Customer
	st.Plan = p.Plan
	st.MaxUnits = p.MaxUnits
	exp := p.ExpiresAt
	st.ExpiresAt = &exp

	switch {
	case now.After(p.ExpiresAt.AddDate(0, 0, s.graceDays)):
		st.State = StateExpired
		st.Message = fmt.Sprintf(
			"Lisensi berakhir pada %s. Perpanjang lisensi untuk melanjutkan.",
			p.ExpiresAt.Format("02-01-2006"))

	case now.After(p.ExpiresAt):
		graceLeft := int(math.Ceil(
			p.ExpiresAt.AddDate(0, 0, s.graceDays).Sub(now).Hours() / 24))
		if graceLeft < 0 {
			graceLeft = 0
		}
		st.State = StateGrace
		st.CanStartBusiness = true
		st.CanBindUnit = st.MaxUnits == 0 || st.UnitsUsed < st.MaxUnits
		st.Message = fmt.Sprintf(
			"Lisensi sudah berakhir. Masa tenggang %d hari lagi.", graceLeft)

	default:
		left := int(math.Ceil(p.ExpiresAt.Sub(now).Hours() / 24))
		if left < 0 {
			left = 0
		}
		st.DaysLeft = left
		st.CanStartBusiness = true
		st.CanBindUnit = st.MaxUnits == 0 || st.UnitsUsed < st.MaxUnits
		if left <= 30 {
			st.State = StateExpiring
			st.ExpiringSoon = true
			st.Message = fmt.Sprintf(
				"Lisensi berakhir dalam %d hari (%s). Segera perpanjang.",
				left, p.ExpiresAt.Format("02-01-2006"))
		} else {
			st.State = StateActive
			st.Message = fmt.Sprintf("Lisensi aktif sampai %s.",
				p.ExpiresAt.Format("02-01-2006"))
		}
	}

	if st.MaxUnits > 0 && st.UnitsUsed >= st.MaxUnits && st.CanStartBusiness {
		st.Warnings = append(st.Warnings, fmt.Sprintf(
			"Semua unit lisensi sudah terpakai (%d/%d).", st.UnitsUsed, st.MaxUnits))
	}
	return st
}

// EnsureCanBindUnit menolak pemasangan TV baru bila lisensi tidak mengizinkan.
func (s *Service) EnsureCanBindUnit(ctx context.Context) error {
	st := s.Status(ctx)
	if !st.CanStartBusiness {
		return errors.New(st.Message)
	}
	used := s.unitsUsed(ctx)
	if st.MaxUnits > 0 && used >= st.MaxUnits {
		return fmt.Errorf(
			"Batas unit lisensi tercapai (%d/%d). Upgrade lisensi untuk menambah TV.",
			used, st.MaxUnits)
	}
	return nil
}

// EnsureCanStartBusiness menolak transaksi baru saat lisensi sudah berakhir.
func (s *Service) EnsureCanStartBusiness(ctx context.Context) error {
	st := s.Status(ctx)
	if !st.CanStartBusiness {
		return errors.New(st.Message)
	}
	return nil
}

// Activate memverifikasi dan menyimpan lisensi baru.
// Bila lisensi baru menurunkan masa berlaku/unit, perlu force=true.
func (s *Service) Activate(ctx context.Context, licenseText string, force bool) (*Status, []string, error) {
	if s.db == nil {
		return nil, nil, errors.New("database tidak tersedia")
	}
	if len(s.publicKey) != ed25519.PublicKeySize {
		return nil, nil, errors.New("public key lisensi belum dikonfigurasi pada server")
	}

	p, err := licensekey.Verify(licenseText, s.publicKey)
	if err != nil {
		return nil, nil, err
	}
	ident := s.identity(ctx)
	if ident.ID == uuid.Nil {
		return nil, nil, errors.New("gagal membaca identitas server")
	}
	if err := licensekey.ValidateStructure(p, ident.ID); err != nil {
		return nil, nil, err
	}

	now := time.Now()
	if !p.ExpiresAt.After(now) {
		return nil, nil, errors.New("lisensi sudah kedaluwarsa")
	}
	if p.IssuedAt.IsZero() {
		p.IssuedAt = now
	}

	// Bandingkan dengan lisensi aktif untuk mencegah penurunan tak sengaja.
	warnings := make([]string, 0, 3)
	var cur activeRow
	_ = s.db.WithContext(ctx).Raw(
		"SELECT * FROM " + spname.Ident("GetActiveLicense") + "()").Scan(&cur).Error

	if cur.ID != uuid.Nil && cur.ExpiresAt.After(now) {
		if p.ExpiresAt.Before(cur.ExpiresAt) {
			warnings = append(warnings, fmt.Sprintf(
				"Masa berlaku lisensi baru (%s) lebih pendek dari lisensi saat ini (%s); sisa waktu akan hilang.",
				p.ExpiresAt.Format("02-01-2006"), cur.ExpiresAt.Format("02-01-2006")))
		}
		if p.MaxUnits != 0 && cur.MaxUnits != 0 && p.MaxUnits < cur.MaxUnits {
			warnings = append(warnings, fmt.Sprintf(
				"Jumlah unit lisensi baru (%d) lebih sedikit dari lisensi saat ini (%d).",
				p.MaxUnits, cur.MaxUnits))
		}
	}

	used := s.unitsUsed(ctx)
	if p.MaxUnits > 0 && used > p.MaxUnits {
		warnings = append(warnings, fmt.Sprintf(
			"Saat ini %d TV terhubung, melebihi lisensi baru (%d unit). Lepaskan sebagian TV agar sesuai.",
			used, p.MaxUnits))
	}

	if len(warnings) > 0 && !force {
		return nil, warnings, ErrNeedsConfirmation
	}

	raw, _ := json.Marshal(p)
	var saved struct {
		ID uuid.UUID `gorm:"column:id"`
	}
	err = s.db.WithContext(ctx).Raw(
		"SELECT * FROM "+spname.Ident("SaveLicense")+
			"(?::text, ?::varchar, ?::varchar, ?::varchar, ?::int, ?::timestamptz, ?::timestamptz, ?::uuid, ?::jsonb)",
		licenseText, p.LicenseID, p.Customer, p.Plan, p.MaxUnits,
		p.IssuedAt, p.ExpiresAt, ident.ID, string(raw),
	).Scan(&saved).Error
	if err != nil {
		return nil, nil, fmt.Errorf("gagal menyimpan lisensi: %w", err)
	}

	detail, _ := json.Marshal(map[string]any{
		"plan":      p.Plan,
		"customer":  p.Customer,
		"maxUnits":  p.MaxUnits,
		"expiresAt": p.ExpiresAt,
		"parent":    p.ParentLicenseID,
	})
	_ = s.db.WithContext(ctx).Exec(
		"SELECT "+spname.Ident("LogLicenseEvent")+"(?::varchar, ?::varchar, ?::jsonb)",
		"activated", p.LicenseID, string(detail)).Error

	s.invalidate()
	return s.Status(ctx), warnings, nil
}

// History mengembalikan riwayat lisensi yang pernah diaktifkan.
func (s *Service) History(ctx context.Context, limit int) ([]HistoryRow, error) {
	if s.db == nil {
		return nil, errors.New("database tidak tersedia")
	}
	var rows []HistoryRow
	err := s.db.WithContext(ctx).Raw(
		"SELECT * FROM "+spname.Ident("LicenseHistory")+"(?)", limit).Scan(&rows).Error
	if rows == nil {
		rows = []HistoryRow{}
	}
	return rows, err
}
