package usecase

import (
	"context"
	"errors"
	"time"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/domain/repository"

	"github.com/google/uuid"
)

// SessionUseCase mendefinisikan logika bisnis untuk manajemen sesi rental
type SessionUseCase interface {
	GetAllSessions(ctx context.Context) ([]*entity.Session, error)
	GetSessionByID(ctx context.Context, id uuid.UUID) (*entity.Session, error)
	GetActiveSessions(ctx context.Context) ([]*entity.Session, error)
	StartSession(ctx context.Context, req *StartSessionRequest) (*StartSessionResponse, error)
	EndSession(ctx context.Context, id uuid.UUID) (*entity.Session, error)
	CancelSession(ctx context.Context, id uuid.UUID) error
}

// SetupGraceMinutes adalah waktu persiapan konsol yang diberikan gratis di
// awal sesi (tidak menambah tagihan dan tidak memicu notifikasi).
const SetupGraceMinutes = 2

type sessionUseCase struct {
	sessionRepo repository.SessionRepository
	consoleRepo repository.ConsoleRepository
}

// NewSessionUseCase membuat instance baru SessionUseCase
func NewSessionUseCase(sessionRepo repository.SessionRepository, consoleRepo repository.ConsoleRepository) SessionUseCase {
	return &sessionUseCase{
		sessionRepo: sessionRepo,
		consoleRepo: consoleRepo,
	}
}

// StartSessionRequest payload untuk memulai sesi rental baru dengan pembayaran di depan
type StartSessionRequest struct {
	ConsoleID  uuid.UUID  `json:"consoleId"  validate:"required"   example:"550e8400-e29b-41d4-a716-446655440000"`
	CustomerID *uuid.UUID `json:"customerId"                        example:"550e8400-e29b-41d4-a716-446655440001"` // opsional, walk-in tidak perlu
	// Durasi yang dipesan dalam menit. Boleh 0 kalau pakai bank waktu / tukar poin.
	BookedDurationMinutes int `json:"bookedDurationMinutes" validate:"omitempty,min=0" example:"30"`
	// Uang tunai yang diberikan pelanggan di depan. Boleh 0 kalau sesi gratis (bank waktu / poin).
	CashReceived float64 `json:"cashReceived" validate:"omitempty,min=0" example:"25000"`
	// Kode voucher diskon (opsional)
	VoucherCode string `json:"voucherCode" example:"DISKON10"`
	Notes       string `json:"notes" example:"Rental 2 jam"`
	// Menit dari saldo bank waktu member yang ingin dipakai sebagai bonus (opsional, hanya untuk member, gratis tidak dihitung ke pembayaran)
	UseTimeBankMinutes int `json:"useTimeBankMinutes" validate:"omitempty,min=0" example:"0"`
	// Jumlah unit penukaran poin loyalitas (opsional, hanya untuk member). 1 unit = 1x threshold poin ditukar jadi menit bonus, sesuai app_settings
	RedeemPointUnits int `json:"redeemPointUnits" validate:"omitempty,min=0" example:"0"`
	// Gunakan bonus waktu promo (mis. weekend 30 menit) bila memenuhi syarat.
	// Bonus tidak menambah tagihan, hanya menambah menit bermain.
	UsePromoBonus bool `json:"usePromoBonus" example:"false"`
	// Metode pembayaran: cash (default), qris, atau transfer.
	PaymentMethod string `json:"paymentMethod" validate:"omitempty,oneof=cash qris transfer" example:"cash"`
	// CreatedBy diisi server dari token, bukan client.
	CreatedBy *uuid.UUID `json:"-"`
}

// StartSessionResponse respons setelah memulai sesi — berisi data sesi dan pembayaran lunas
type StartSessionResponse struct {
	Session *entity.Session `json:"session"`
	Payment *entity.Payment `json:"payment"`
}

func (uc *sessionUseCase) GetAllSessions(ctx context.Context) ([]*entity.Session, error) {
	return uc.sessionRepo.FindAll(ctx)
}

func (uc *sessionUseCase) GetSessionByID(ctx context.Context, id uuid.UUID) (*entity.Session, error) {
	session, err := uc.sessionRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, errors.New("sesi tidak ditemukan")
	}
	return session, nil
}

func (uc *sessionUseCase) GetActiveSessions(ctx context.Context) ([]*entity.Session, error) {
	return uc.sessionRepo.FindActiveSession(ctx)
}

func (uc *sessionUseCase) StartSession(ctx context.Context, req *StartSessionRequest) (*StartSessionResponse, error) {
	// Cek apakah konsol tersedia
	console, err := uc.consoleRepo.FindByID(ctx, req.ConsoleID)
	if err != nil {
		return nil, err
	}
	if console == nil {
		return nil, errors.New("konsol tidak ditemukan")
	}
	if !console.IsAvailable() {
		return nil, errors.New("konsol tidak tersedia untuk disewa saat ini")
	}

	// Cek apakah sudah ada sesi aktif di konsol ini
	activeSession, err := uc.sessionRepo.FindActiveByConsoleID(ctx, req.ConsoleID)
	if err != nil {
		return nil, err
	}
	if activeSession != nil {
		return nil, errors.New("konsol masih memiliki sesi aktif")
	}

	now := time.Now()
	session := &entity.Session{
		ID:                    uuid.New(),
		ConsoleID:             req.ConsoleID,
		CustomerID:            req.CustomerID,
		StartTime:             now,
		BookedDurationMinutes: req.BookedDurationMinutes,
		Status:                entity.SessionStatusActive,
		Notes:                 req.Notes,
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	// Buat sesi + pembayaran di depan dalam satu transaksi atomik
	payment, err := uc.sessionRepo.CreateWithPayment(ctx, session, req.CashReceived, req.VoucherCode, req.UseTimeBankMinutes, req.RedeemPointUnits, req.PaymentMethod, req.CreatedBy)
	if err != nil {
		return nil, err
	}

	// Beri waktu persiapan konsol di AWAL sesi: start_time dan end_scheduled_at
	// sama-sama digeser, sehingga jam sesi baru berjalan setelah setup selesai.
	// Menit berbayar & harga tetap, dan tidak ada notifikasi khusus untuk ini.
	if updated, graceErr := uc.sessionRepo.AddSetupGrace(ctx, session.ID, SetupGraceMinutes); graceErr == nil && updated != nil {
		session = updated
	} else {
		session.Console = console
	}
	return &StartSessionResponse{Session: session, Payment: payment}, nil
}

func (uc *sessionUseCase) EndSession(ctx context.Context, id uuid.UUID) (*entity.Session, error) {
	session, err := uc.sessionRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, errors.New("sesi tidak ditemukan")
	}
	if session.Status != entity.SessionStatusActive {
		return nil, errors.New("sesi sudah tidak aktif")
	}

	console, err := uc.consoleRepo.FindByID(ctx, session.ConsoleID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	session.EndTime = &now
	session.DurationMinutes = session.CalculateDuration()
	session.TotalPrice = session.CalculateTotalPrice(console.PricePerHour)
	session.Status = entity.SessionStatusCompleted
	session.UpdatedAt = now

	// EndSession SP + reset konsol dijalankan atomik oleh repository.
	if err := uc.sessionRepo.Update(ctx, session); err != nil {
		return nil, err
	}

	session.Console = console
	return session, nil
}

func (uc *sessionUseCase) CancelSession(ctx context.Context, id uuid.UUID) error {
	session, err := uc.sessionRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if session == nil {
		return errors.New("sesi tidak ditemukan")
	}
	if session.Status != entity.SessionStatusActive {
		return errors.New("hanya sesi aktif yang dapat dibatalkan")
	}

	// CancelSession SP + reset konsol dijalankan atomik oleh repository.
	return uc.sessionRepo.UpdateStatus(ctx, id, entity.SessionStatusCancelled)
}
