package repository

import (
	"context"

	"byone-arena/internal/domain/entity"

	"github.com/google/uuid"
)

// CapitalRepository — akses data pembukuan modal, seluruhnya via stored procedure.
type CapitalRepository interface {
	AddEntry(ctx context.Context, entry *entity.CapitalEntry) error
	DeleteEntry(ctx context.Context, id uuid.UUID) error
	ListEntries(ctx context.Context, startDate, endDate string) ([]*entity.CapitalEntry, error)
	MonthlyReport(ctx context.Context, months int) ([]entity.MonthlyCapital, error)
}
