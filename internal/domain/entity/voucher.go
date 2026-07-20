package entity

import (
	"time"

	"github.com/google/uuid"
)

// DiscountType mendefinisikan jenis diskon voucher
type DiscountType string

const (
	DiscountTypePercentage  DiscountType = "percentage"   // diskon persen dari total
	DiscountTypeFixedAmount DiscountType = "fixed_amount" // diskon nominal tetap
	DiscountTypeFreeDays    DiscountType = "free_days"    // gratis N hari pada rental harian
)

// Voucher merepresentasikan kode diskon yang dapat digunakan saat pembayaran
type Voucher struct {
	ID            uuid.UUID    `json:"id"            gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	Code          string       `json:"code"          gorm:"not null;uniqueIndex;size:50"`
	Name          string       `json:"name"          gorm:"not null;size:150"`
	DiscountType  DiscountType `json:"discountType"  gorm:"not null;size:20"`
	DiscountValue float64      `json:"discountValue" gorm:"not null;type:numeric(10,2)"`
	MinPurchase   float64      `json:"minPurchase"   gorm:"not null;default:0;type:numeric(10,2)"`  // minimal total sebelum voucher berlaku
	MaxDiscount   float64      `json:"maxDiscount"   gorm:"not null;default:0;type:numeric(10,2)"`  // batas maks diskon persen (0 = tidak terbatas)
	MaxUsage      int          `json:"maxUsage"      gorm:"not null;default:0"`                     // batas total pemakaian (0 = tidak terbatas)
	UsageCount    int          `json:"usageCount"    gorm:"not null;default:0"`
	IsActive      bool         `json:"isActive"      gorm:"not null;default:true"`
	ExpiresAt     *time.Time   `json:"expiresAt,omitempty"`
	CreatedAt     time.Time    `json:"createdAt"     gorm:"autoCreateTime"`
	UpdatedAt     time.Time    `json:"updatedAt"     gorm:"autoUpdateTime"`
}

func (Voucher) TableName() string { return "vouchers" }
