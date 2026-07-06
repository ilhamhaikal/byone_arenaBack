package repository

import (
	"context"

	"byone-arena/internal/domain/entity"

	"github.com/google/uuid"
)

// PaymentRepository mendefinisikan kontrak akses data untuk entitas Payment
type PaymentRepository interface {
	FindAll(ctx context.Context) ([]*entity.Payment, error)
	FindByID(ctx context.Context, id uuid.UUID) (*entity.Payment, error)
	FindBySessionID(ctx context.Context, sessionID uuid.UUID) (*entity.Payment, error)
	Create(ctx context.Context, payment *entity.Payment) error
	Update(ctx context.Context, payment *entity.Payment) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status entity.PaymentStatus) error
	// GetDashboardSummary mengambil ringkasan pendapatan untuk dashboard.
	// date: format YYYY-MM-DD (default hari ini jika kosong)
	GetDashboardSummary(ctx context.Context, date string) (*entity.DashboardSummary, error)
	// GetReportSummary mengambil laporan komprehensif untuk rentang tanggal.
	GetReportSummary(ctx context.Context, startDate, endDate string) (*entity.ReportSummary, error)
}
