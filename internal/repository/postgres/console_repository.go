package postgres

import (
	"context"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/domain/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type consoleRepository struct {
	db *gorm.DB
}

// NewConsoleRepository membuat instance baru ConsoleRepository berbasis GORM
func NewConsoleRepository(db *gorm.DB) repository.ConsoleRepository {
	return &consoleRepository{db: db}
}

func (r *consoleRepository) FindAll(ctx context.Context) ([]*entity.Console, error) {
	var consoles []*entity.Console
	result := r.db.WithContext(ctx).Order("name ASC").Find(&consoles)
	return consoles, result.Error
}

func (r *consoleRepository) FindByID(ctx context.Context, id uuid.UUID) (*entity.Console, error) {
	var console entity.Console
	result := r.db.WithContext(ctx).First(&console, "id = ?", id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return &console, nil
}

func (r *consoleRepository) FindByStatus(ctx context.Context, status entity.ConsoleStatus) ([]*entity.Console, error) {
	var consoles []*entity.Console
	result := r.db.WithContext(ctx).Where("status = ?", status).Order("name ASC").Find(&consoles)
	return consoles, result.Error
}

func (r *consoleRepository) Create(ctx context.Context, console *entity.Console) error {
	return r.db.WithContext(ctx).Create(console).Error
}

func (r *consoleRepository) Update(ctx context.Context, console *entity.Console) error {
	return r.db.WithContext(ctx).Save(console).Error
}

func (r *consoleRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status entity.ConsoleStatus) error {
	return r.db.WithContext(ctx).
		Model(&entity.Console{}).
		Where("id = ?", id).
		Update("status", status).Error
}

func (r *consoleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&entity.Console{}, "id = ?", id).Error
}

