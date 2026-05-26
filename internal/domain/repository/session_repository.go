package repository

import (
	"context"

	"byone-arena/internal/domain/entity"

	"github.com/google/uuid"
)

// SessionRepository mendefinisikan kontrak akses data untuk entitas Session
type SessionRepository interface {
	FindAll(ctx context.Context) ([]*entity.Session, error)
	FindByID(ctx context.Context, id uuid.UUID) (*entity.Session, error)
	FindByConsoleID(ctx context.Context, consoleID uuid.UUID) ([]*entity.Session, error)
	FindActiveByConsoleID(ctx context.Context, consoleID uuid.UUID) (*entity.Session, error)
	FindByCustomerID(ctx context.Context, customerID uuid.UUID) ([]*entity.Session, error)
	FindActiveSession(ctx context.Context) ([]*entity.Session, error)
	Create(ctx context.Context, session *entity.Session) error
	// CreateWithPayment membuat sesi + pembayaran pre-pay dalam satu transaksi atomik.
	// Mengembalikan entitas Payment yang sudah terisi (amount, diskon, kembalian, dll).
	CreateWithPayment(ctx context.Context, session *entity.Session, cashReceived float64, voucherCode string) (*entity.Payment, error)
	Update(ctx context.Context, session *entity.Session) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status entity.SessionStatus) error
	Delete(ctx context.Context, id uuid.UUID) error
}
