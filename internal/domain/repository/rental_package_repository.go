package repository

import (
	"context"

	"byone-arena/internal/domain/entity"

	"github.com/google/uuid"
)

// RentalPackageItemInput — satu baris item bundling untuk input.
type RentalPackageItemInput struct {
	MenuItemID uuid.UUID
	Quantity   int
}

// RentalPackageRepository — akses data paket rental, seluruhnya via stored procedure.
type RentalPackageRepository interface {
	List(ctx context.Context, includeInactive bool) ([]*entity.RentalPackage, error)
	Save(ctx context.Context, pkg *entity.RentalPackage, items []RentalPackageItemInput) (uuid.UUID, error)
	Delete(ctx context.Context, id uuid.UUID) error
	StartSessionWithPackage(ctx context.Context, consoleID uuid.UUID, customerID *uuid.UUID, packageID uuid.UUID, cashReceived float64, notes string) (*entity.Session, *entity.Payment, error)
}
