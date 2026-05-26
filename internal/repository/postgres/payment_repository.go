package postgres

import (
	"context"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/domain/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type paymentRepository struct {
	db *gorm.DB
}

// NewPaymentRepository membuat instance baru PaymentRepository berbasis GORM + Stored Procedure
func NewPaymentRepository(db *gorm.DB) repository.PaymentRepository {
	return &paymentRepository{db: db}
}

func (r *paymentRepository) FindAll(ctx context.Context) ([]*entity.Payment, error) {
	var payments []*entity.Payment
	result := r.db.WithContext(ctx).Preload("Session").Order("created_at DESC").Find(&payments)
	return payments, result.Error
}

func (r *paymentRepository) FindByID(ctx context.Context, id uuid.UUID) (*entity.Payment, error) {
	var payment entity.Payment
	result := r.db.WithContext(ctx).Preload("Session").First(&payment, "id = ?", id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return &payment, nil
}

func (r *paymentRepository) FindBySessionID(ctx context.Context, sessionID uuid.UUID) (*entity.Payment, error) {
	var payment entity.Payment
	result := r.db.WithContext(ctx).Where("session_id = ?", sessionID).First(&payment)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return &payment, nil
}

// Create menggunakan stored procedure sp_create_payment untuk validasi dan atomisitas
func (r *paymentRepository) Create(ctx context.Context, payment *entity.Payment) error {
	type spResult struct {
		PaymentID      uuid.UUID  `gorm:"column:payment_id"`
		Amount         float64    `gorm:"column:amount"`
		DiscountAmount float64    `gorm:"column:discount_amount"`
		CashReceived   float64    `gorm:"column:cash_received"`
		ChangeAmount   float64    `gorm:"column:change_amount"`
		VoucherID      *uuid.UUID `gorm:"column:voucher_id"`
	}

	var result spResult
	tx := r.db.WithContext(ctx).Raw(
		"SELECT * FROM sp_create_payment(?, ?, ?, ?)",
		payment.SessionID,
		payment.CashReceived,
		payment.Notes,
		payment.VoucherCode, // field sementara, tidak di-persist langsung
	).Scan(&result)

	if tx.Error != nil {
		return parseStoredProcError(tx.Error)
	}

	payment.ID = result.PaymentID
	payment.Amount = result.Amount
	payment.DiscountAmount = result.DiscountAmount
	payment.CashReceived = result.CashReceived
	payment.ChangeAmount = result.ChangeAmount
	payment.VoucherID = result.VoucherID
	payment.PaymentStatus = entity.PaymentStatusPaid
	return nil
}

func (r *paymentRepository) Update(ctx context.Context, payment *entity.Payment) error {
	return r.db.WithContext(ctx).Save(payment).Error
}

func (r *paymentRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status entity.PaymentStatus) error {
	if status == entity.PaymentStatusRefunded {
		// Gunakan stored procedure untuk refund
		tx := r.db.WithContext(ctx).Exec("SELECT sp_refund_payment(?)", id)
		return parseStoredProcError(tx.Error)
	}
	return r.db.WithContext(ctx).
		Model(&entity.Payment{}).
		Where("id = ?", id).
		Update("payment_status", status).Error
}

