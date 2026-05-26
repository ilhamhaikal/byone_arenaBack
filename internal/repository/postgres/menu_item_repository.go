package postgres

import (
	"context"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/domain/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type menuItemRepository struct {
	db *gorm.DB
}

// NewMenuItemRepository membuat instance baru MenuItemRepository
func NewMenuItemRepository(db *gorm.DB) repository.MenuItemRepository {
	return &menuItemRepository{db: db}
}

func (r *menuItemRepository) FindAll(ctx context.Context) ([]*entity.MenuItem, error) {
	var items []*entity.MenuItem
	if err := r.db.WithContext(ctx).Order("category ASC, name ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *menuItemRepository) FindAvailable(ctx context.Context) ([]*entity.MenuItem, error) {
	var items []*entity.MenuItem
	if err := r.db.WithContext(ctx).
		Where("is_available = TRUE").
		Order("category ASC, name ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *menuItemRepository) FindByID(ctx context.Context, id uuid.UUID) (*entity.MenuItem, error) {
	var item entity.MenuItem
	if err := r.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *menuItemRepository) FindByCategory(ctx context.Context, category entity.MenuItemCategory) ([]*entity.MenuItem, error) {
	var items []*entity.MenuItem
	if err := r.db.WithContext(ctx).
		Where("category = ? AND is_available = TRUE", category).
		Order("name ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *menuItemRepository) Create(ctx context.Context, item *entity.MenuItem) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *menuItemRepository) Update(ctx context.Context, item *entity.MenuItem) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *menuItemRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&entity.MenuItem{}, "id = ?", id).Error
}
