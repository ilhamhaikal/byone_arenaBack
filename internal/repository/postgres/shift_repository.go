package postgres

import (
	"context"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/domain/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type shiftRepository struct {
	db *gorm.DB
}

// NewShiftRepository membuat instance baru ShiftRepository berbasis GORM
func NewShiftRepository(db *gorm.DB) repository.ShiftRepository {
	return &shiftRepository{db: db}
}

func (r *shiftRepository) FindAll(ctx context.Context) ([]*entity.Shift, error) {
	var shifts []*entity.Shift
	result := r.db.WithContext(ctx).Preload("User").Order("created_at DESC").Find(&shifts)
	return shifts, result.Error
}

func (r *shiftRepository) FindByID(ctx context.Context, id uuid.UUID) (*entity.Shift, error) {
	var shift entity.Shift
	result := r.db.WithContext(ctx).Preload("User").First(&shift, "id = ?", id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return &shift, nil
}

func (r *shiftRepository) FindByUserID(ctx context.Context, userID uuid.UUID) ([]*entity.Shift, error) {
	var shifts []*entity.Shift
	result := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&shifts)
	return shifts, result.Error
}

func (r *shiftRepository) FindActiveByUserID(ctx context.Context, userID uuid.UUID) ([]*entity.Shift, error) {
	var shifts []*entity.Shift
	result := r.db.WithContext(ctx).
		Where("user_id = ? AND status = 'active'", userID).
		Find(&shifts)
	return shifts, result.Error
}

func (r *shiftRepository) Create(ctx context.Context, shift *entity.Shift) error {
	return r.db.WithContext(ctx).Create(shift).Error
}

func (r *shiftRepository) Update(ctx context.Context, shift *entity.Shift) error {
	return r.db.WithContext(ctx).Save(shift).Error
}

func (r *shiftRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&entity.Shift{}, "id = ?", id).Error
}
