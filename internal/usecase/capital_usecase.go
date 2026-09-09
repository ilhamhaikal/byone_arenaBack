package usecase

import (
	"context"
	"errors"
	"time"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/domain/repository"

	"github.com/google/uuid"
)

// CapitalUseCase — logika bisnis pembukuan modal awal.
type CapitalUseCase interface {
	AddEntry(ctx context.Context, req *AddCapitalEntryRequest) (*entity.CapitalEntry, error)
	DeleteEntry(ctx context.Context, id uuid.UUID) error
	ListEntries(ctx context.Context, startDate, endDate string) ([]*entity.CapitalEntry, error)
	MonthlyReport(ctx context.Context, months int) ([]entity.MonthlyCapital, error)
}

// AddCapitalEntryRequest payload untuk mencatat modal awal.
type AddCapitalEntryRequest struct {
	Amount    float64 `json:"amount"    validate:"required,gt=0"`
	EntryDate string  `json:"entryDate" validate:"required"` // format YYYY-MM-DD
	Notes     string  `json:"notes"`
}

type capitalUseCase struct {
	repo repository.CapitalRepository
}

// NewCapitalUseCase membuat instance baru CapitalUseCase
func NewCapitalUseCase(repo repository.CapitalRepository) CapitalUseCase {
	return &capitalUseCase{repo: repo}
}

func (uc *capitalUseCase) AddEntry(ctx context.Context, req *AddCapitalEntryRequest) (*entity.CapitalEntry, error) {
	d, err := time.Parse("2006-01-02", req.EntryDate)
	if err != nil {
		return nil, errors.New("entryDate harus berformat YYYY-MM-DD")
	}
	entry := &entity.CapitalEntry{Amount: req.Amount, EntryDate: d, Notes: req.Notes}
	if err := uc.repo.AddEntry(ctx, entry); err != nil {
		return nil, err
	}
	return entry, nil
}

func (uc *capitalUseCase) DeleteEntry(ctx context.Context, id uuid.UUID) error {
	return uc.repo.DeleteEntry(ctx, id)
}

func (uc *capitalUseCase) ListEntries(ctx context.Context, startDate, endDate string) ([]*entity.CapitalEntry, error) {
	return uc.repo.ListEntries(ctx, startDate, endDate)
}

func (uc *capitalUseCase) MonthlyReport(ctx context.Context, months int) ([]entity.MonthlyCapital, error) {
	rows, err := uc.repo.MonthlyReport(ctx, months)
	if err != nil {
		return nil, err
	}
	// Hitung persentase perubahan & alasan (vs bulan sebelumnya, input ASC).
	for i := range rows {
		if i > 0 {
			prev := rows[i-1]
			rows[i].PercentChange = percentChange(rows[i].NetProfit, prev.NetProfit)
			rows[i].Reason = monthlyReason(rows[i])
		}
	}
	return rows, nil
}

func percentChange(cur, prev float64) float64 {
	if prev == 0 {
		if cur > 0 {
			return 100
		}
		return 0
	}
	return (cur - prev) / absFloat(prev) * 100
}

func absFloat(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

func monthlyReason(m entity.MonthlyCapital) string {
	if m.InitialCapital > 0 && m.TotalRevenue < m.InitialCapital {
		return "Pembelanjaan bahan belum kembali modal dari penjualan"
	}
	if m.FoodRevenue < m.InitialCapital {
		return "Pendapatan makanan masih di bawah modal awal beli bahan"
	}
	return "Modal awal sudah kembali modal dari pendapatan"
}
