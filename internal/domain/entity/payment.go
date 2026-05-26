package entity

import (
	"time"

	"github.com/google/uuid"
)

// PaymentMethod mendefinisikan metode pembayaran
// Saat ini hanya mendukung pembayaran tunai (cash)
type PaymentMethod string

const (
	PaymentMethodCash PaymentMethod = "cash"
)

// PaymentStatus mendefinisikan status pembayaran
type PaymentStatus string

const (
	PaymentStatusPending  PaymentStatus = "pending"
	PaymentStatusPaid     PaymentStatus = "paid"
	PaymentStatusRefunded PaymentStatus = "refunded"
)

// Payment merepresentasikan transaksi pembayaran tunai dari satu sesi rental
type Payment struct {
	ID             uuid.UUID     `json:"id"              gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	SessionID      uuid.UUID     `json:"sessionId"       gorm:"type:uuid;not null;uniqueIndex"`
	VoucherID      *uuid.UUID    `json:"voucherId,omitempty" gorm:"type:uuid;index"`
	Amount         float64       `json:"amount"          gorm:"not null;type:numeric(10,2)"`
	DiscountAmount float64       `json:"discountAmount"  gorm:"not null;default:0;type:numeric(10,2)"` // nominal diskon yang diberikan
	PaymentMethod  PaymentMethod `json:"paymentMethod"   gorm:"not null;default:'cash';size:20"`
	PaymentStatus  PaymentStatus `json:"paymentStatus"   gorm:"not null;default:'pending';size:20;index"`
	PaidAt         *time.Time    `json:"paidAt,omitempty"`
	CashReceived        float64       `json:"cashReceived"        gorm:"not null;default:0;type:numeric(10,2)"` // uang yang diterima
	ChangeAmount        float64       `json:"changeAmount"        gorm:"not null;default:0;type:numeric(10,2)"` // kembalian
	AutoDiscountAmount  float64       `json:"autoDiscountAmount" gorm:"not null;default:0;type:numeric(10,2)"` // diskon otomatis (happy hour, member, dll)
	Notes               string        `json:"notes,omitempty"    gorm:"type:text"`
	CreatedAt      time.Time     `json:"createdAt"       gorm:"autoCreateTime"`
	UpdatedAt      time.Time     `json:"updatedAt"       gorm:"autoUpdateTime"`

	// Relasi
	Session *Session `json:"session,omitempty"  gorm:"foreignKey:SessionID"`
	Voucher *Voucher `json:"voucher,omitempty"  gorm:"foreignKey:VoucherID"`

	// Field transient — hanya untuk dikirim ke stored procedure, tidak disimpan langsung
	VoucherCode string `json:"-" gorm:"-"`
}

func (Payment) TableName() string { return "payments" }
