package entity

import (
	"time"

	"github.com/google/uuid"
)

// RuleType mendefinisikan jenis kondisi diskon otomatis
type RuleType string

const (
	RuleTypeAlways     RuleType = "always"      // berlaku untuk semua transaksi
	RuleTypeHappyHour  RuleType = "happy_hour"  // berlaku pada jam tertentu
	RuleTypeMember     RuleType = "member"       // khusus pelanggan member
	RuleTypeDayOfWeek  RuleType = "day_of_week" // berlaku pada hari tertentu
)

// DiscountRule merepresentasikan aturan diskon otomatis yang dievaluasi saat pembayaran
type DiscountRule struct {
	ID            uuid.UUID    `json:"id"            gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	Name          string       `json:"name"          gorm:"not null;size:150"`
	RuleType      RuleType     `json:"ruleType"      gorm:"not null;size:20"`
	DiscountType  DiscountType `json:"discountType"  gorm:"not null;size:20"` // reuse dari voucher.go
	DiscountValue float64      `json:"discountValue" gorm:"not null;type:numeric(10,2)"`
	MaxDiscount   float64      `json:"maxDiscount"   gorm:"not null;default:0;type:numeric(10,2)"` // batas maks diskon persen (0 = tidak terbatas)
	MinPurchase   float64      `json:"minPurchase"   gorm:"not null;default:0;type:numeric(10,2)"` // minimal total sebelum rule berlaku
	// Happy hour: jam mulai dan jam selesai (0-23, bisa lintas tengah malam)
	StartHour    *int   `json:"startHour,omitempty"   gorm:"type:smallint"`
	EndHour      *int   `json:"endHour,omitempty"     gorm:"type:smallint"`
	// Day of week: comma-separated "0,1,2" (0=Minggu, 1=Senin, ..., 6=Sabtu)
	DaysOfWeek   string `json:"daysOfWeek,omitempty"  gorm:"size:20"`
	Priority     int    `json:"priority"              gorm:"not null;default:0"` // lebih besar = lebih dulu dievaluasi
	IsActive     bool   `json:"isActive"              gorm:"not null;default:true"`
	CreatedAt    time.Time `json:"createdAt"         gorm:"autoCreateTime"`
	UpdatedAt    time.Time `json:"updatedAt"         gorm:"autoUpdateTime"`
}

func (DiscountRule) TableName() string { return "discount_rules" }
