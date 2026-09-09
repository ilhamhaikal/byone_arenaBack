package postgres

import (
	"context"
	"fmt"
	"time"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/domain/repository"
	"byone-arena/pkg/spname"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type capitalRepository struct {
	db *gorm.DB
}

// NewCapitalRepository membuat instance baru CapitalRepository
func NewCapitalRepository(db *gorm.DB) repository.CapitalRepository {
	return &capitalRepository{db: db}
}

func (r *capitalRepository) AddEntry(ctx context.Context, entry *entity.CapitalEntry) error {
	type spResult struct {
		ID        uuid.UUID `gorm:"column:id"`
		Amount    float64   `gorm:"column:amount"`
		EntryDate time.Time `gorm:"column:entry_date"`
		Notes     string    `gorm:"column:notes"`
		CreatedAt time.Time `gorm:"column:created_at"`
	}
	var res spResult
	tx := r.db.WithContext(ctx).Raw(
		fmt.Sprintf("SELECT * FROM %s(?, ?, ?)", spname.Ident("AddCapitalEntry")),
		entry.Amount, entry.EntryDate, entry.Notes,
	).Scan(&res)
	if tx.Error != nil {
		return tx.Error
	}
	entry.ID = res.ID
	entry.Amount = res.Amount
	entry.EntryDate = res.EntryDate
	entry.Notes = res.Notes
	entry.CreatedAt = res.CreatedAt
	return nil
}

func (r *capitalRepository) DeleteEntry(ctx context.Context, id uuid.UUID) error {
	tx := r.db.WithContext(ctx).Exec(
		fmt.Sprintf("SELECT %s(?)", spname.Ident("DeleteCapitalEntry")), id,
	)
	if tx.Error != nil {
		return parseStoredProcError(tx.Error)
	}
	return nil
}

func (r *capitalRepository) ListEntries(ctx context.Context, startDate, endDate string) ([]*entity.CapitalEntry, error) {
	type spRow struct {
		ID        uuid.UUID `gorm:"column:id"`
		Amount    float64   `gorm:"column:amount"`
		EntryDate time.Time `gorm:"column:entry_date"`
		Notes     string    `gorm:"column:notes"`
		CreatedAt time.Time `gorm:"column:created_at"`
	}
	var rows []spRow
	tx := r.db.WithContext(ctx).Raw(
		fmt.Sprintf("SELECT * FROM %s(?::DATE, ?::DATE)", spname.Ident("ListCapitalEntries")),
		startDate, endDate,
	).Scan(&rows)
	if tx.Error != nil {
		return nil, tx.Error
	}
	out := make([]*entity.CapitalEntry, 0, len(rows))
	for _, rw := range rows {
		out = append(out, &entity.CapitalEntry{
			ID:        rw.ID,
			Amount:    rw.Amount,
			EntryDate: rw.EntryDate,
			Notes:     rw.Notes,
			CreatedAt: rw.CreatedAt,
		})
	}
	return out, nil
}

func (r *capitalRepository) MonthlyReport(ctx context.Context, months int) ([]entity.MonthlyCapital, error) {
	type spRow struct {
		Month          string  `gorm:"column:month"`
		InitialCapital float64 `gorm:"column:initial_capital"`
		TotalRevenue   float64 `gorm:"column:total_revenue"`
		FoodRevenue    float64 `gorm:"column:food_revenue"`
		NetProfit      float64 `gorm:"column:net_profit"`
	}
	var rows []spRow
	tx := r.db.WithContext(ctx).Raw(
		fmt.Sprintf("SELECT * FROM %s(?)", spname.Ident("MonthlyCapitalReport")), months,
	).Scan(&rows)
	if tx.Error != nil {
		return nil, tx.Error
	}
	out := make([]entity.MonthlyCapital, 0, len(rows))
	for _, rw := range rows {
		out = append(out, entity.MonthlyCapital{
			Month:          rw.Month,
			InitialCapital: rw.InitialCapital,
			TotalRevenue:   rw.TotalRevenue,
			FoodRevenue:    rw.FoodRevenue,
			NetProfit:      rw.NetProfit,
		})
	}
	return out, nil
}
