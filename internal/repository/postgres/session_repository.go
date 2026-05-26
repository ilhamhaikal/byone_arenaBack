package postgres

import (
	"context"
	"fmt"
	"strings"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/domain/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type sessionRepository struct {
	db *gorm.DB
}

// NewSessionRepository membuat instance baru SessionRepository berbasis GORM + Stored Procedure
func NewSessionRepository(db *gorm.DB) repository.SessionRepository {
	return &sessionRepository{db: db}
}

func (r *sessionRepository) FindAll(ctx context.Context) ([]*entity.Session, error) {
	var sessions []*entity.Session
	result := r.db.WithContext(ctx).
		Preload("Console").
		Preload("Customer").
		Order("created_at DESC").
		Find(&sessions)
	return sessions, result.Error
}

func (r *sessionRepository) FindByID(ctx context.Context, id uuid.UUID) (*entity.Session, error) {
	var session entity.Session
	result := r.db.WithContext(ctx).
		Preload("Console").
		Preload("Customer").
		First(&session, "id = ?", id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return &session, nil
}

func (r *sessionRepository) FindByConsoleID(ctx context.Context, consoleID uuid.UUID) ([]*entity.Session, error) {
	var sessions []*entity.Session
	result := r.db.WithContext(ctx).
		Preload("Console").
		Where("console_id = ?", consoleID).
		Order("created_at DESC").
		Find(&sessions)
	return sessions, result.Error
}

func (r *sessionRepository) FindActiveByConsoleID(ctx context.Context, consoleID uuid.UUID) (*entity.Session, error) {
	var session entity.Session
	result := r.db.WithContext(ctx).
		Where("console_id = ? AND status = 'active'", consoleID).
		First(&session)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return &session, nil
}

func (r *sessionRepository) FindByCustomerID(ctx context.Context, customerID uuid.UUID) ([]*entity.Session, error) {
	var sessions []*entity.Session
	result := r.db.WithContext(ctx).
		Preload("Console").
		Where("customer_id = ?", customerID).
		Order("created_at DESC").
		Find(&sessions)
	return sessions, result.Error
}

func (r *sessionRepository) FindActiveSession(ctx context.Context) ([]*entity.Session, error) {
	var sessions []*entity.Session
	result := r.db.WithContext(ctx).
		Preload("Console").
		Preload("Customer").
		Where("status = 'active'").
		Order("start_time ASC").
		Find(&sessions)
	return sessions, result.Error
}

// Create menggunakan stored procedure sp_start_session untuk atomisitas
func (r *sessionRepository) Create(ctx context.Context, session *entity.Session) error {
	type spResult struct {
		ID              uuid.UUID  `gorm:"column:id"`
		ConsoleID       uuid.UUID  `gorm:"column:console_id"`
		CustomerID      *uuid.UUID `gorm:"column:customer_id"`
		StartTime       interface{} `gorm:"column:start_time"`
		EndTime         interface{} `gorm:"column:end_time"`
		DurationMinutes int        `gorm:"column:duration_minutes"`
		TotalPrice      float64    `gorm:"column:total_price"`
		Status          string     `gorm:"column:status"`
		Notes           string     `gorm:"column:notes"`
		CreatedAt       interface{} `gorm:"column:created_at"`
		UpdatedAt       interface{} `gorm:"column:updated_at"`
	}

	var result spResult
	tx := r.db.WithContext(ctx).Raw(
		"SELECT * FROM sp_start_session(?, ?, ?)",
		session.ConsoleID,
		session.CustomerID,
		session.Notes,
	).Scan(&result)

	if tx.Error != nil {
		return parseStoredProcError(tx.Error)
	}

	session.ID = result.ID
	session.Status = entity.SessionStatus(result.Status)
	return nil
}

// Update menggunakan stored procedure sp_end_session
func (r *sessionRepository) Update(ctx context.Context, session *entity.Session) error {
	// Update hanya dipanggil saat end session → gunakan stored procedure
	tx := r.db.WithContext(ctx).Exec("SELECT sp_end_session(?)", session.ID)
	if tx.Error != nil {
		return parseStoredProcError(tx.Error)
	}
	// Refresh data sesi setelah update
	return r.db.WithContext(ctx).First(session, "id = ?", session.ID).Error
}

// UpdateStatus menggunakan GORM langsung (fallback untuk cancel)
func (r *sessionRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status entity.SessionStatus) error {
	if status == entity.SessionStatusCancelled {
		// Gunakan stored procedure untuk membatalkan (atomic dengan update konsol)
		tx := r.db.WithContext(ctx).Exec("SELECT sp_cancel_session(?)", id)
		return parseStoredProcError(tx.Error)
	}
	// Fallback GORM untuk status lain
	return r.db.WithContext(ctx).
		Model(&entity.Session{}).
		Where("id = ?", id).
		Update("status", status).Error
}

func (r *sessionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&entity.Session{}, "id = ?", id).Error
}

// parseStoredProcError mengkonversi error PostgreSQL dari stored procedure
// menjadi error Go yang lebih deskriptif
func parseStoredProcError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	// Format error dari RAISE EXCEPTION: "ERROR: <prefix>: <message> (SQLSTATE P0001)"
	for _, prefix := range []string{
		"CONSOLE_NOT_FOUND", "CONSOLE_NOT_AVAILABLE", "SESSION_ALREADY_ACTIVE",
		"SESSION_NOT_FOUND", "SESSION_NOT_ACTIVE",
		"SESSION_NOT_COMPLETED", "PAYMENT_EXISTS", "INSUFFICIENT_CASH",
		"PAYMENT_NOT_FOUND", "PAYMENT_NOT_PAID",
	} {
		if strings.Contains(msg, prefix+":") {
			start := strings.Index(msg, prefix+":")
			end := strings.Index(msg[start:], " (SQLSTATE")
			if end == -1 {
				return fmt.Errorf("%s", msg[start+len(prefix)+2:])
			}
			return fmt.Errorf("%s", strings.TrimSpace(msg[start+len(prefix)+2:start+end]))
		}
	}
	return err
}

