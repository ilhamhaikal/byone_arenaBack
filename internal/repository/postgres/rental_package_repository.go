package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/domain/repository"
	"byone-arena/pkg/spname"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type rentalPackageRepository struct {
	db *gorm.DB
}

// NewRentalPackageRepository membuat instance baru RentalPackageRepository
func NewRentalPackageRepository(db *gorm.DB) repository.RentalPackageRepository {
	return &rentalPackageRepository{db: db}
}

type packageRow struct {
	ID              uuid.UUID       `gorm:"column:id"`
	Name            string          `gorm:"column:name"`
	StartTime       string          `gorm:"column:start_time"`
	EndTime         string          `gorm:"column:end_time"`
	Price           float64         `gorm:"column:price"`
	DurationMinutes int             `gorm:"column:duration_minutes"`
	Description     string          `gorm:"column:description"`
	IsActive        bool            `gorm:"column:is_active"`
	CreatedAt       time.Time       `gorm:"column:created_at"`
	UpdatedAt       time.Time       `gorm:"column:updated_at"`
	Items           json.RawMessage `gorm:"column:items"`
}

func (r *rentalPackageRepository) List(ctx context.Context, includeInactive bool) ([]*entity.RentalPackage, error) {
	var rows []packageRow
	tx := r.db.WithContext(ctx).Raw(
		fmt.Sprintf("SELECT * FROM %s(?)", spname.Ident("ListRentalPackages")), includeInactive,
	).Scan(&rows)
	if tx.Error != nil {
		return nil, tx.Error
	}
	out := make([]*entity.RentalPackage, 0, len(rows))
	for _, rw := range rows {
		out = append(out, toPackage(rw))
	}
	return out, nil
}

func (r *rentalPackageRepository) Save(ctx context.Context, pkg *entity.RentalPackage, items []repository.RentalPackageItemInput) (uuid.UUID, error) {
	type itemJSON struct {
		MenuItemID string `json:"menuItemId"`
		Quantity   int    `json:"quantity"`
	}
	arr := make([]itemJSON, 0, len(items))
	for _, it := range items {
		arr = append(arr, itemJSON{MenuItemID: it.MenuItemID.String(), Quantity: it.Quantity})
	}
	b, _ := json.Marshal(arr)

	// Alias kolom hasil fungsi sebagai "id" supaya GORM scan ke struct dengan
	// aman (uuid.UUID sebagai destination langsung memicu scan error karena
	// direfleksikan sebagai array of uint8).
	type spID struct {
		ID uuid.UUID `gorm:"column:id"`
	}
	var res spID
	tx := r.db.WithContext(ctx).Raw(
		fmt.Sprintf("SELECT %s(?, ?, ?::TIME, ?::TIME, ?, ?, ?, ?::JSONB) AS id", spname.Ident("SaveRentalPackage")),
		pkg.ID, pkg.Name, pkg.StartTime, pkg.EndTime, pkg.Price, pkg.Description, pkg.IsActive, string(b),
	).Scan(&res)
	if tx.Error != nil {
		return uuid.Nil, parseStoredProcError(tx.Error)
	}
	return res.ID, nil
}

func (r *rentalPackageRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tx := r.db.WithContext(ctx).Exec(fmt.Sprintf("SELECT %s(?)", spname.Ident("DeleteRentalPackage")), id)
	if tx.Error != nil {
		return parseStoredProcError(tx.Error)
	}
	return nil
}

type packageSessionRow struct {
	SessionID        uuid.UUID  `gorm:"column:session_id"`
	SessionStatus    string     `gorm:"column:session_status"`
	SessionStartTime *time.Time `gorm:"column:session_start_time"`
	SessionBookedMin int        `gorm:"column:session_booked_minutes"`
	SessionEndSched  *time.Time `gorm:"column:session_end_scheduled"`
	PaymentID        uuid.UUID  `gorm:"column:payment_id"`
	TotalPayment     float64    `gorm:"column:total_payment"`
	CashReceived     float64    `gorm:"column:cash_received"`
	ChangeAmount     float64    `gorm:"column:change_amount"`
	PaidAt           *time.Time `gorm:"column:paid_at"`
	PackageName      string     `gorm:"column:package_name"`
}

func (r *rentalPackageRepository) StartSessionWithPackage(ctx context.Context, consoleID uuid.UUID, customerID *uuid.UUID, packageID uuid.UUID, cashReceived float64, notes string, createdBy *uuid.UUID) (*entity.Session, *entity.Payment, error) {
	var res packageSessionRow
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Raw(
			fmt.Sprintf("SELECT * FROM %s(?, ?, ?, ?, ?)", spname.Ident("StartSessionWithPackage")),
			consoleID, customerID, packageID, cashReceived, notes,
		).Scan(&res).Error; err != nil {
			return parseStoredProcError(err)
		}
		if createdBy != nil {
			if err := tx.Model(&entity.Session{}).
				Where("id = ?", res.SessionID).
				Update("created_by", *createdBy).Error; err != nil {
				return err
			}
			return tx.Model(&entity.Payment{}).
				Where("id = ?", res.PaymentID).
				Update("created_by", *createdBy).Error
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	var startTime time.Time
	if res.SessionStartTime != nil {
		startTime = *res.SessionStartTime
	}
	session := &entity.Session{
		ID:                    res.SessionID,
		ConsoleID:             consoleID,
		CustomerID:            customerID,
		StartTime:             startTime,
		BookedDurationMinutes: res.SessionBookedMin,
		EndScheduledAt:        res.SessionEndSched,
		TotalPrice:            res.TotalPayment,
		Status:                entity.SessionStatus(res.SessionStatus),
		Notes:                 notes,
	}
	payment := &entity.Payment{
		ID:            res.PaymentID,
		SessionID:     res.SessionID,
		Amount:        res.TotalPayment,
		TotalPayment:  res.TotalPayment,
		CashReceived:  res.CashReceived,
		ChangeAmount:  res.ChangeAmount,
		PaymentMethod: entity.PaymentMethodCash,
		PaymentStatus: entity.PaymentStatusPaid,
	}
	return session, payment, nil
}

func toPackage(rw packageRow) *entity.RentalPackage {
	p := &entity.RentalPackage{
		ID:              rw.ID,
		Name:            rw.Name,
		Price:           rw.Price,
		DurationMinutes: rw.DurationMinutes,
		Description:     rw.Description,
		IsActive:        rw.IsActive,
		CreatedAt:       rw.CreatedAt,
		UpdatedAt:       rw.UpdatedAt,
		StartTime:       trimSeconds(rw.StartTime),
		EndTime:         trimSeconds(rw.EndTime),
	}
	var items []entity.RentalPackageItem
	if err := json.Unmarshal(rw.Items, &items); err == nil {
		p.Items = items
	} else {
		p.Items = []entity.RentalPackageItem{}
	}
	return p
}

func trimSeconds(t string) string {
	if len(t) >= 5 {
		return t[:5]
	}
	return t
}
