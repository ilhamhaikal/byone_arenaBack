package entity

import "time"

const (
	// PromoAudienceGeneral dipakai untuk pelanggan umum / tanpa data pelanggan.
	PromoAudienceGeneral = "general"
	// PromoAudienceMember hanya dipakai bila pelanggan valid sebagai member.
	PromoAudienceMember = "member"
)

// PromoBonusRule mengatur bonus waktu gratis, mis. akhir pekan:
// main minimal 60 menit gratis 30 menit.
//
// DaysOfWeek memakai format DOW PostgreSQL: 0=Minggu .. 6=Sabtu, dipisah koma.
// Nilai 7 diterima sebagai alias Minggu untuk kompatibilitas data lama.
type PromoBonusRule struct {
	ID             string `json:"id"              gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	Name           string `json:"name"            gorm:"size:100;not null"`
	DaysOfWeek     string `json:"daysOfWeek"      gorm:"size:20;not null;default:'0,6'"`
	MinPaidMinutes int    `json:"minPaidMinutes"  gorm:"not null;default:60"`
	BonusMinutes   int    `json:"bonusMinutes"    gorm:"not null;default:30"`
	// Audience: general atau member.
	Audience string `json:"audience" gorm:"size:20;not null;default:'general'"`
	// MaxPerSession 0 = boleh berulang tanpa batas; >0 = maksimal per sesi.
	MaxPerSession int        `json:"maxPerSession"   gorm:"not null;default:0"`
	ValidFrom     *time.Time `json:"validFrom,omitempty" gorm:"type:date"`
	ValidTo       *time.Time `json:"validTo,omitempty"   gorm:"type:date"`
	IsActive      bool       `json:"isActive"        gorm:"not null;default:true"`
	CreatedAt     time.Time  `json:"createdAt"       gorm:"autoCreateTime"`
	UpdatedAt     time.Time  `json:"updatedAt"       gorm:"autoUpdateTime"`
}

func (PromoBonusRule) TableName() string { return "promo_bonus_rules" }
