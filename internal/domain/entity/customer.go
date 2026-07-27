package entity

import (
	"time"

	"github.com/google/uuid"
)

// Customer merepresentasikan pelanggan yang terdaftar
type Customer struct {
	ID        uuid.UUID `json:"id"           gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	Name      string    `json:"name"         gorm:"not null;size:100"`
	Phone     string    `json:"phone"        gorm:"uniqueIndex;not null;size:20"`
	Email     string    `json:"email,omitempty" gorm:"size:150"`
	IsMember  bool      `json:"isMember"     gorm:"not null;default:false"`
	// Membership fields
	MembershipType  *string `json:"membershipType,omitempty"  gorm:"size:20"`       // monthly, yearly, lifetime
	MembershipStart *string `json:"membershipStart,omitempty" gorm:"type:date"`      // YYYY-MM-DD
	MembershipExpiry *string `json:"membershipExpiry,omitempty" gorm:"type:date"`    // YYYY-MM-DD
	MembershipPrice float64 `json:"membershipPrice"           gorm:"not null;default:0;type:numeric(10,2)"`
	// Bank Waktu & Poin Loyalitas (khusus member)
	TimeBalanceMinutes int `json:"timeBalanceMinutes" gorm:"not null;default:0"` // sisa menit dari sesi yang diakhiri lebih cepat, bisa dipakai lagi
	LoyaltyPoints      int `json:"loyaltyPoints"      gorm:"not null;default:0"` // poin dari transaksi bayar-di-depan, bisa ditukar menit gratis
	CreatedAt time.Time `json:"createdAt"    gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt"    gorm:"autoUpdateTime"`
}

func (Customer) TableName() string { return "customers" }
