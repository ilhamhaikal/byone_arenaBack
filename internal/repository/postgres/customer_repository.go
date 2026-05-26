package postgres

import (
	"context"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/domain/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type customerRepository struct {
	db *gorm.DB
}

// NewCustomerRepository membuat instance baru CustomerRepository berbasis GORM
func NewCustomerRepository(db *gorm.DB) repository.CustomerRepository {
	return &customerRepository{db: db}
}

func (r *customerRepository) FindAll(ctx context.Context) ([]*entity.Customer, error) {
	var customers []*entity.Customer
	result := r.db.WithContext(ctx).Order("name ASC").Find(&customers)
	return customers, result.Error
}

func (r *customerRepository) FindByID(ctx context.Context, id uuid.UUID) (*entity.Customer, error) {
	var customer entity.Customer
	result := r.db.WithContext(ctx).First(&customer, "id = ?", id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return &customer, nil
}

func (r *customerRepository) FindByPhone(ctx context.Context, phone string) (*entity.Customer, error) {
	var customer entity.Customer
	result := r.db.WithContext(ctx).Where("phone = ?", phone).First(&customer)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return &customer, nil
}

func (r *customerRepository) Create(ctx context.Context, customer *entity.Customer) error {
	return r.db.WithContext(ctx).Create(customer).Error
}

func (r *customerRepository) Update(ctx context.Context, customer *entity.Customer) error {
	return r.db.WithContext(ctx).Save(customer).Error
}

func (r *customerRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&entity.Customer{}, "id = ?", id).Error
}

