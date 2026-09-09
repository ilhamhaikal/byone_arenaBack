package entity

import (
	"time"

	"github.com/google/uuid"
)

// CapitalEntry — satu catatan modal awal (pengeluaran beli bahan) pada tanggal tertentu.
type CapitalEntry struct {
	ID        uuid.UUID `json:"id"         gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	Amount    float64   `json:"amount"     gorm:"not null;type:numeric(12,2)"`
	EntryDate time.Time `json:"entryDate"  gorm:"not null;type:date"`
	Notes     string    `json:"notes,omitempty" gorm:"type:text"`
	CreatedAt time.Time `json:"createdAt"  gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt"  gorm:"autoUpdateTime"`
}

func (CapitalEntry) TableName() string { return "capital_entries" }

// MonthlyCapital — perbandingan modal vs pendapatan per bulan.
type MonthlyCapital struct {
	Month          string  `json:"month"`
	InitialCapital float64 `json:"initialCapital"`
	TotalRevenue   float64 `json:"totalRevenue"`
	FoodRevenue    float64 `json:"foodRevenue"`
	NetProfit      float64 `json:"netProfit"`
	PercentChange  float64 `json:"percentChange"`
	Reason         string  `json:"reason"`
}
