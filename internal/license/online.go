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
	State    string   `json:"state"`
	Valid    bool     `json:"valid"`
	Message  string   `json:"message,omitempty"`
	Status   *Status  `json:"status,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// pendingOnlineActivation hanya hidup di memori proses. Database billing tidak
// menyimpan secret/code aktivasi; jika server restart, buat kode baru.
type pendingOnlineActivation struct {
	Code             string
	Secret           string
	ExpiresAt        time.Time
	PollAfterSeconds int
}

// ConfigureOnline mengaktifkan integrasi license server vendor.
func (s *Service) ConfigureOnline(vendor VendorAPI, appVersion string, heartbeatEvery time.Duration) {
	s.onlineMu.Lock()
	defer s.onlineMu.Unlock()
	s.vendor = vendor
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
	return s.vendor != nil && s.vendor.Enabled()
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

	now := time.Now()
	if s.pendingOnline != nil && now.Before(s.pendingOnline.ExpiresAt) {
		return &OnlineActivationStart{
			Code: s.pendingOnline.Code, ExpiresAt: s.pendingOnline.ExpiresAt,
			PollAfterSeconds: s.pendingOnline.PollAfterSeconds, InstanceID: ident.ID.String(),
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
	pollAfter := created.PollAfterSeconds
	if pollAfter < 2 || pollAfter > 30 {
		pollAfter = 5
	}
	s.pendingOnline = &pendingOnlineActivation{
		Code: created.Code, Secret: created.Secret,
		ExpiresAt: created.ExpiresAt, PollAfterSeconds: pollAfter,
	}
	s.logLicenseEvent(ctx, "online_requested", "", map[string]any{
		"code": created.Code, "instanceId": ident.ID.String(),
	})
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

	pending := s.pendingOnline
	if pending == nil {
		return nil, errors.New("tidak ada aktivasi tertunda; buat kode baru")
	}
	now := time.Now()
	if now.After(pending.ExpiresAt) {
		s.pendingOnline = nil
		return &OnlineActivationPoll{
			State: "expired", Message: "Kode aktivasi sudah kedaluwarsa. Buat kode baru.",
		}, nil
	}
	result, err := s.vendor.PollActivationRequest(ctx, pending.Code, pending.Secret)
	if err != nil {
		return &OnlineActivationPoll{State: "error", Message: err.Error()}, nil
	}

	switch {
	case result.Valid && result.License != "":
		status, warnings, err := s.Activate(ctx, result.License, true)
		if err != nil {
			return &OnlineActivationPoll{State: "error", Message: err.Error()}, nil
		}
		s.pendingOnline = nil
		s.logLicenseEvent(ctx, "online_activated", result.LicenseID, map[string]any{
			"code": pending.Code, "plan": result.Plan,
		})
		return &OnlineActivationPoll{
			State: "activated", Valid: true, Status: status, Warnings: warnings,
			Message: "Lisensi berhasil diaktifkan.",
		}, nil

	case result.Status == "rejected" || result.Code == "rejected":
		s.pendingOnline = nil
		return &OnlineActivationPoll{State: "rejected", Message: result.Message}, nil

	case result.Status == "expired" || result.Code == "expired":
		s.pendingOnline = nil
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

// HeartbeatNow melakukan satu siklus heartbeat. State revocation disimpan di
// memori proses; yang dipersistenkan ke database hanya token lisensinya.
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
		"SELECT * FROM " + spname.Ident("GetActiveLicense") + "()").Scan(&row).Error; err != nil {
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
		return err
	}
	at := result.ServerTime
	if at.IsZero() {
		at = now
	}

	if !result.Valid {
		state := remoteStateFor(result.Code)
		if state != "" {
			s.setRemoteStatus(state, result.Message, at)
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
		"SELECT * FROM "+spname.Ident("UpdateLicenseToken")+
			"(?::text, ?::varchar, ?::varchar, ?::varchar, ?::int, ?::timestamptz, ?::jsonb, ?::timestamptz)",
		result.License, payload.LicenseID, payload.Customer, payload.Plan,
		payload.MaxUnits, payload.ExpiresAt, string(raw), at).Scan(&saved).Error; err != nil {
		return fmt.Errorf("gagal menyimpan token heartbeat: %w", err)
	}
	s.setRemoteStatus("", "", at)
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
