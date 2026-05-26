package usecase

import (
	"context"
	"errors"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/domain/repository"

	"github.com/google/uuid"
)

// PaymentUseCase mendefinisikan logika bisnis untuk manajemen pembayaran tunai
type PaymentUseCase interface {
	GetPaymentByID(ctx context.Context, id uuid.UUID) (*entity.Payment, error)
	GetPaymentBySessionID(ctx context.Context, sessionID uuid.UUID) (*entity.Payment, error)
	CreateCashPayment(ctx context.Context, req *CreateCashPaymentRequest) (*entity.Payment, error)
	RefundPayment(ctx context.Context, id uuid.UUID) (*entity.Payment, error)
}

type paymentUseCase struct {
	paymentRepo repository.PaymentRepository
	sessionRepo repository.SessionRepository
}

// NewPaymentUseCase membuat instance baru PaymentUseCase
func NewPaymentUseCase(paymentRepo repository.PaymentRepository, sessionRepo repository.SessionRepository) PaymentUseCase {
	return &paymentUseCase{
		paymentRepo: paymentRepo,
		sessionRepo: sessionRepo,
	}
}

// CreateCashPaymentRequest payload untuk membuat pembayaran tunai
type CreateCashPaymentRequest struct {
	SessionID    uuid.UUID `json:"sessionId"    validate:"required"`
	CashReceived float64   `json:"cashReceived" validate:"required,gt=0"`
	VoucherCode  string    `json:"voucherCode"`  // opsional — kode diskon voucher
	Notes        string    `json:"notes"`
}

func (uc *paymentUseCase) GetPaymentByID(ctx context.Context, id uuid.UUID) (*entity.Payment, error) {
	payment, err := uc.paymentRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if payment == nil {
		return nil, errors.New("data pembayaran tidak ditemukan")
	}
	return payment, nil
}

func (uc *paymentUseCase) GetPaymentBySessionID(ctx context.Context, sessionID uuid.UUID) (*entity.Payment, error) {
	return uc.paymentRepo.FindBySessionID(ctx, sessionID)
}

// CreateCashPayment membuat pembayaran tunai melalui stored procedure
// Validasi, diskon voucher, perhitungan kembalian, dan update status dilakukan di SP
func (uc *paymentUseCase) CreateCashPayment(ctx context.Context, req *CreateCashPaymentRequest) (*entity.Payment, error) {
	payment := &entity.Payment{
		SessionID:     req.SessionID,
		PaymentMethod: entity.PaymentMethodCash,
		CashReceived:  req.CashReceived,
		VoucherCode:   req.VoucherCode,
		Notes:         req.Notes,
	}

	// sp_create_payment menangani: validasi sesi, apply voucher, hitung kembalian, set status paid
	if err := uc.paymentRepo.Create(ctx, payment); err != nil {
		return nil, err
	}
	return payment, nil
}

func (uc *paymentUseCase) RefundPayment(ctx context.Context, id uuid.UUID) (*entity.Payment, error) {
	payment, err := uc.paymentRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if payment == nil {
		return nil, errors.New("data pembayaran tidak ditemukan")
	}

	// sp_refund_payment menangani validasi status
	if err := uc.paymentRepo.UpdateStatus(ctx, id, entity.PaymentStatusRefunded); err != nil {
		return nil, err
	}

	payment.PaymentStatus = entity.PaymentStatusRefunded
	return payment, nil
}

