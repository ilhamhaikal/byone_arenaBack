package license

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"byone-arena/pkg/licensekey"
	"byone-arena/pkg/spname"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// OnlineActivationStart adalah kode aktivasi yang ditampilkan sebagai QR.
type OnlineActivationStart struct {
	Code             string    `json:"code"`
	ExpiresAt        time.Time `json:"expiresAt"`
	PollAfterSeconds int       `json:"pollAfterSeconds"`
	InstanceID       string    `json:"instanceId"`
}

// OnlineActivationPoll adalah hasil pengecekan status aktivasi.
type OnlineActivationPoll struct {
	State    string   `json:"state"` // waiting, activated, rejected, expired, error
	Valid    bool     `json:"valid"`
	Message  string   `json:"message,omitempty"`
	Status   *Status  `json:"status,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// ConfigureOnline mengaktifkan integrasi license server vendor.
// Tanpa ini, aplikasi tetap berjalan dengan lisensi offline/manual.
func (s *Service) ConfigureOnline(vendor VendorAPI, serverSecret, appVersion string, heartbeatEvery time.Duration) {
	s.onlineMu.Lock()
	defer s.onlineMu.Unlock()
	s.vendor = vendor
	s.secretBox = newSecretBox(serverSecret)
	s.appVersion = strings.TrimSpace(appVersion)
	if s.appVersion == "" {
		s.appVersion = "1.0.4"
	}
	if heartbeatEvery < 5*time.Minute {
		heartbeatEvery = 30 * time.Minute
	}
	s.heartbeatEvery = heartbeatEvery
}

func (s *Service) onlineEnabled() bool {
	return s.vendor != nil && s.vendor.Enabled() && s.secretBox != nil
}

type pendingActivationRow struct {
	ID           uuid.UUID `gorm:"column:id"`
	Code         string    `gorm:"column:code"`
	SecretCipher string    `gorm:"column:secret_cipher"`
	ExpiresAt    time.Time `gorm:"column:expires_at"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}

func (s *Service) pendingActivation(ctx context.Context) (pendingActivationRow, error) {
	var row pendingActivationRow
	if s.db == nil {
		return row, errors.New("database tidak tersedia")
	}
	err := s.db.WithContext(ctx).Raw(
		"SELECT * FROM " + spname.Ident("GetPendingLicenseActivationRequest") + "()").Scan(&row).Error
	return row, err
}

func (s *Service) markActivation(ctx context.Context, id uuid.UUID, status string, at time.Time) error {
	if s.db == nil {
		return errors.New("database tidak tersedia")
	}
	return s.db.WithContext(ctx).Exec(
		"SELECT "+spname.Ident("UpdateLicenseActivationRequest")+"(?::uuid, ?::varchar, ?::timestamptz)",
		id, status, at).Error
}

func (s *Service) logLicenseEvent(ctx context.Context, event, licenseID string, detail map[string]any) {
	if s.db == nil {
		return
	}
	raw, _ := json.Marshal(detail)
	_ = s.db.WithContext(ctx).Exec(
		"SELECT "+spname.Ident("LogLicenseEvent")+"(?::varchar, ?::varchar, ?::jsonb)",
		event, licenseID, string(raw)).Error
}

// StartOnlineActivation membuat kode aktivasi QR di license server vendor.
// Jika masih ada request pending, kode yang sama dikembalikan agar aman saat
// aplikasi ditutup lalu dibuka kembali.
func (s *Service) StartOnlineActivation(ctx context.Context, machineLabel, osInfo string) (*OnlineActivationStart, error) {
	if !s.onlineEnabled() {
		return nil, ErrOnlineDisabled
	}
	s.onlineMu.Lock()
	defer s.onlineMu.Unlock()

	ident := s.identity(ctx)
	if ident.ID == uuid.Nil {
		return nil, errors.New("gagal membaca identitas server")
	}

	pending, err := s.pendingActivation(ctx)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca request aktivasi: %w", err)
	}
	if pending.ID != uuid.Nil && time.Now().Before(pending.ExpiresAt) {
		return &OnlineActivationStart{
			Code: pending.Code, ExpiresAt: pending.ExpiresAt,
			PollAfterSeconds: 5, InstanceID: ident.ID.String(),
		}, nil
	}

	machineLabel = strings.TrimSpace(machineLabel)
	if machineLabel == "" {
		if hostname, err := os.Hostname(); err == nil {
			machineLabel = hostname
		}
	}
	if machineLabel == "" {
		machineLabel = "Server billing"
	}
	osInfo = strings.TrimSpace(osInfo)
	if osInfo == "" {
		osInfo = runtime.GOOS + " " + runtime.GOARCH
	}

	created, err := s.vendor.CreateActivationRequest(ctx, ident.ID.String(), machineLabel, osInfo)
	if err != nil {
		return nil, err
	}
	secretCipher, err := s.secretBox.seal(created.Secret)
	if err != nil {
		return nil, fmt.Errorf("gagal mengamankan secret aktivasi: %w", err)
	}
	if s.db == nil {
		return nil, errors.New("database tidak tersedia")
	}
	var saved struct {
		ID uuid.UUID `gorm:"column:id"`
	}
	if err := s.db.WithContext(ctx).Raw(
		"SELECT * FROM "+spname.Ident("StartLicenseActivationRequest")+"(?::varchar, ?::text, ?::timestamptz)",
		created.Code, secretCipher, created.ExpiresAt).Scan(&saved).Error; err != nil {
		return nil, fmt.Errorf("gagal menyimpan request aktivasi: %w", err)
	}
	s.logLicenseEvent(ctx, "online_requested", "", map[string]any{
		"code": created.Code, "instanceId": ident.ID.String(),
	})

	pollAfter := created.PollAfterSeconds
	if pollAfter < 2 || pollAfter > 30 {
		pollAfter = 5
	}
	return &OnlineActivationStart{
		Code: created.Code, ExpiresAt: created.ExpiresAt,
		PollAfterSeconds: pollAfter, InstanceID: ident.ID.String(),
	}, nil
}

// PollOnlineActivation memeriksa vendor lalu menyimpan token lisensi bila sudah
// diterbitkan. Secret tidak pernah dikirim ke frontend.
func (s *Service) PollOnlineActivation(ctx context.Context) (*OnlineActivationPoll, error) {
	if !s.onlineEnabled() {
		return nil, ErrOnlineDisabled
	}
	s.onlineMu.Lock()
	defer s.onlineMu.Unlock()

	pending, err := s.pendingActivation(ctx)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca request aktivasi: %w", err)
	}
	if pending.ID == uuid.Nil {
		return nil, errors.New("tidak ada aktivasi tertunda; buat kode baru")
	}
	now := time.Now()
	if now.After(pending.ExpiresAt) {
		_ = s.markActivation(ctx, pending.ID, "expired", now)
		return &OnlineActivationPoll{
			State: "expired", Message: "Kode aktivasi sudah kedaluwarsa. Buat kode baru.",
		}, nil
	}
	secret, err := s.secretBox.open(pending.SecretCipher)
	if err != nil {
		return nil, err
	}
	result, err := s.vendor.PollActivationRequest(ctx, pending.Code, secret)
	if err != nil {
		_ = s.markActivation(ctx, pending.ID, "pending", now)
		return &OnlineActivationPoll{State: "error", Message: err.Error()}, nil
	}
	_ = s.markActivation(ctx, pending.ID, "pending", now)

	switch {
	case result.Valid && result.License != "":
		status, warnings, err := s.Activate(ctx, result.License, true)
		if err != nil {
			return &OnlineActivationPoll{State: "error", Message: err.Error()}, nil
		}
		_ = s.markActivation(ctx, pending.ID, "claimed", now)
		s.logLicenseEvent(ctx, "online_activated", result.LicenseID, map[string]any{
			"code": pending.Code, "plan": result.Plan,
		})
		return &OnlineActivationPoll{
			State: "activated", Valid: true, Status: status, Warnings: warnings,
			Message: "Lisensi berhasil diaktifkan.",
		}, nil

	case result.Status == "rejected" || result.Code == "rejected":
		_ = s.markActivation(ctx, pending.ID, "rejected", now)
		return &OnlineActivationPoll{State: "rejected", Message: result.Message}, nil

	case result.Status == "expired" || result.Code == "expired":
		_ = s.markActivation(ctx, pending.ID, "expired", now)
		return &OnlineActivationPoll{State: "expired", Message: result.Message}, nil

	default:
		message := result.Message
		if message == "" {
			message = "Menunggu lisensi diterbitkan admin."
		}
		return &OnlineActivationPoll{State: "waiting", Message: message}, nil
	}
}

// StartHeartbeat menjalankan heartbeat periodik selama server hidup.
func (s *Service) StartHeartbeat(ctx context.Context, log *zap.Logger) {
	if !s.onlineEnabled() {
		if log != nil {
			log.Info("Heartbeat lisensi nonaktif: LICENSE_SERVER_URL belum diisi")
		}
		return
	}
	interval := s.heartbeatEvery
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	go func() {
		// Beri waktu server/DB selesai inisialisasi.
		timer := time.NewTimer(15 * time.Second)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				if err := s.HeartbeatNow(ctx); err != nil && log != nil {
					log.Warn("Heartbeat lisensi gagal", zap.Error(err))
				}
				timer.Reset(interval)
			}
		}
	}()
}

// HeartbeatNow melakukan satu siklus heartbeat. Network error tidak mengubah
// status lokal; lisensi tetap berjalan sampai expiry/grace.
func (s *Service) HeartbeatNow(ctx context.Context) error {
	if !s.onlineEnabled() {
		return ErrOnlineDisabled
	}
	s.onlineMu.Lock()
	defer s.onlineMu.Unlock()

	if s.db == nil {
		return errors.New("database tidak tersedia")
	}
	var row activeRow
	if err := s.db.WithContext(ctx).Raw(
		"SELECT * FROM " + spname.Ident("GetActiveLicenseV2") + "()").Scan(&row).Error; err != nil {
		return fmt.Errorf("gagal membaca lisensi aktif: %w", err)
	}
	if row.ID == uuid.Nil || strings.TrimSpace(row.LicenseKey) == "" {
		return nil
	}
	ident := s.identity(ctx)
	if ident.ID == uuid.Nil {
		return errors.New("identitas server tidak tersedia")
	}

	now := time.Now()
	result, err := s.vendor.Heartbeat(ctx, row.LicenseKey, ident.ID.String())
	if err != nil {
		_ = s.db.WithContext(ctx).Exec(
			"SELECT "+spname.Ident("SetLicenseHeartbeatError")+"(?::text, ?::timestamptz)",
			err.Error(), now).Error
		s.invalidate()
		return err
	}
	at := result.ServerTime
	if at.IsZero() {
		at = now
	}

	if !result.Valid {
		state := remoteStateFor(result.Code)
		if state != "" {
			_ = s.db.WithContext(ctx).Exec(
				"SELECT "+spname.Ident("SetLicenseRemoteState")+"(?::varchar, ?::timestamptz, ?::text)",
				state, at, result.Message).Error
			s.invalidate()
		}
		return nil
	}
	if result.License == "" {
		return errors.New("license server tidak mengirim token baru")
	}
	payload, err := licensekey.Verify(result.License, s.publicKey)
	if err != nil {
		return fmt.Errorf("token heartbeat tidak valid: %w", err)
	}
	if err := licensekey.ValidateStructure(payload, ident.ID); err != nil {
		return fmt.Errorf("token heartbeat tidak cocok: %w", err)
	}
	raw, _ := json.Marshal(payload)
	var saved struct {
		ID uuid.UUID `gorm:"column:id"`
	}
	if err := s.db.WithContext(ctx).Raw(
		"SELECT * FROM "+spname.Ident("UpdateLicenseHeartbeat")+
			"(?::text, ?::varchar, ?::varchar, ?::varchar, ?::int, ?::timestamptz, ?::timestamptz, ?::jsonb, ?::timestamptz)",
		result.License, payload.LicenseID, payload.Customer, payload.Plan, payload.MaxUnits,
		payload.IssuedAt, payload.ExpiresAt, string(raw), at).Scan(&saved).Error; err != nil {
		return fmt.Errorf("gagal menyimpan token heartbeat: %w", err)
	}
	s.invalidate()
	return nil
}

func remoteStateFor(code string) string {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case StateSuspended:
		return StateSuspended
	case StateRevoked:
		return StateRevoked
	case StateExpired:
		return StateExpired
	case "activation_inactive", StateInactive:
		return StateInactive
	default:
		return ""
	}
}
