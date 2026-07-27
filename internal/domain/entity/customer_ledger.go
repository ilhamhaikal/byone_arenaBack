package entity

import (
	"time"

	"github.com/google/uuid"
)

// CustomerTimeBankLog mencatat riwayat mutasi saldo bank waktu member
// (earn = kredit sisa waktu dari sesi yang diakhiri lebih cepat,
//  use = pakai saldo saat memulai sesi baru).
type CustomerTimeBankLog struct {
	ID           uuid.UUID  `json:"id"           gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	CustomerID   uuid.UUID  `json:"customerId"   gorm:"type:uuid;not null;index"`
	SessionID    *uuid.UUID `json:"sessionId,omitempty" gorm:"type:uuid"`
	ChangeType   string     `json:"changeType"   gorm:"size:10;not null"` // earn | use
	Minutes      int        `json:"minutes"      gorm:"not null"`
	BalanceAfter int        `json:"balanceAfter" gorm:"not null"`
	CreatedAt    time.Time  `json:"createdAt"    gorm:"autoCreateTime"`
}

func (CustomerTimeBankLog) TableName() string { return "customer_time_bank_logs" }

// CustomerPointLog mencatat riwayat mutasi poin loyalitas member
// (earn = poin didapat dari transaksi bayar-di-depan,
//  redeem = poin ditukar menjadi menit bonus gratis).
type CustomerPointLog struct {
	ID           uuid.UUID  `json:"id"           gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	CustomerID   uuid.UUID  `json:"customerId"   gorm:"type:uuid;not null;index"`
	SessionID    *uuid.UUID `json:"sessionId,omitempty" gorm:"type:uuid"`
	ChangeType   string     `json:"changeType"   gorm:"size:10;not null"` // earn | redeem
	Points       int        `json:"points"       gorm:"not null"`
	BalanceAfter int        `json:"balanceAfter" gorm:"not null"`
	CreatedAt    time.Time  `json:"createdAt"    gorm:"autoCreateTime"`
}

func (CustomerPointLog) TableName() string { return "customer_point_logs" }
