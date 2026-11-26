package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/domain/repository"
	"byone-arena/pkg/paymentmethod"
	"byone-arena/pkg/spname"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

// Create menggunakan byoneStartSession untuk atomisitas
func (r *sessionRepository) Create(ctx context.Context, session *entity.Session) error {
	type spResult struct {
		ID                    uuid.UUID   `gorm:"column:id"`
		ConsoleID             uuid.UUID   `gorm:"column:console_id"`
		CustomerID            *uuid.UUID  `gorm:"column:customer_id"`
		StartTime             interface{} `gorm:"column:start_time"`
		EndTime               interface{} `gorm:"column:end_time"`
		EndScheduledAt        interface{} `gorm:"column:end_scheduled_at"`
		BookedDurationMinutes int         `gorm:"column:booked_duration_minutes"`
		DurationMinutes       int         `gorm:"column:duration_minutes"`
		TotalPrice            float64     `gorm:"column:total_price"`
		Status                string      `gorm:"column:status"`
		Notes                 string      `gorm:"column:notes"`
		CreatedAt             interface{} `gorm:"column:created_at"`
		UpdatedAt             interface{} `gorm:"column:updated_at"`
	}

	var result spResult
	tx := r.db.WithContext(ctx).Raw(
		fmt.Sprintf("SELECT * FROM %s(?, ?, ?, ?)", spname.Ident("StartSession")),
		session.ConsoleID,
		session.CustomerID,
		session.Notes,
		session.BookedDurationMinutes,
	).Scan(&result)

	if tx.Error != nil {
		return parseStoredProcError(tx.Error)
	}

	session.ID = result.ID
	session.BookedDurationMinutes = result.BookedDurationMinutes
	session.Status = entity.SessionStatus(result.Status)
	return nil
}

// CreateWithPayment menggunakan byoneStartSessionWithPayment:
// membuat sesi + pembayaran di depan dalam satu transaksi atomik.
// useTimeBankMinutes & redeemPointUnits opsional (0 = tidak dipakai) — hanya berlaku untuk member.
func (r *sessionRepository) CreateWithPayment(ctx context.Context, session *entity.Session, cashReceived float64, voucherCode string, useTimeBankMinutes int, redeemPointUnits int, paymentMethod string, createdBy *uuid.UUID) (*entity.Payment, error) {
	type spResult struct {
		// sesi
		SessionID           uuid.UUID  `gorm:"column:session_id"`
		SessionStatus       string     `gorm:"column:session_status"`
		SessionStartTime    *time.Time `gorm:"column:session_start_time"`
		SessionBookedMin    int        `gorm:"column:session_booked_minutes"`
		SessionEndScheduled *time.Time `gorm:"column:session_end_scheduled"`
		// pembayaran
		PaymentID          uuid.UUID  `gorm:"column:payment_id"`
		BaseAmount         float64    `gorm:"column:base_amount"`
		DiscountAmount     float64    `gorm:"column:discount_amount"`
		AutoDiscountAmount float64    `gorm:"column:auto_discount_amount"`
		TotalPayment       float64    `gorm:"column:total_payment"`
		CashReceived       float64    `gorm:"column:cash_received"`
		ChangeAmount       float64    `gorm:"column:change_amount"`
		VoucherID          *uuid.UUID `gorm:"column:voucher_id"`
		PaidAt             *time.Time `gorm:"column:paid_at"`
		// bank waktu & poin loyalitas
		PaidDurationMinutes      int `gorm:"column:paid_duration_minutes"`
		TimeBankMinutesUsed      int `gorm:"column:time_bank_minutes_used"`
		BonusMinutesFromPoints   int `gorm:"column:bonus_minutes_from_points"`
		PointsRedeemed           int `gorm:"column:points_redeemed"`
		PointsEarned             int `gorm:"column:points_earned"`
		CustomerTimeBalanceAfter int `gorm:"column:customer_time_balance_after"`
		CustomerPointsAfter      int `gorm:"column:customer_points_after"`
	}

	var result spResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Raw(
			fmt.Sprintf("SELECT * FROM %s(?, ?, ?, ?, ?, ?, ?, ?)", spname.Ident("StartSessionWithPayment")),
			session.ConsoleID,
			session.CustomerID,
			session.Notes,
			session.BookedDurationMinutes,
			cashReceived,
			voucherCode,
			useTimeBankMinutes,
			redeemPointUnits,
		).Scan(&result).Error; err != nil {
			return parseStoredProcError(err)
		}
		if err := paymentmethod.Finalize(tx, result.PaymentID, paymentMethod); err != nil {
			return err
		}
		if session.IsPackage {
			if err := tx.Model(&entity.Session{}).
				Where("id = ?", result.SessionID).
				Update("is_package", true).Error; err != nil {
				return err
			}
		}
		if createdBy != nil {
			if err := tx.Model(&entity.Session{}).
				Where("id = ?", result.SessionID).
				Update("created_by", *createdBy).Error; err != nil {
				return err
			}
			if err := tx.Model(&entity.Payment{}).
				Where("id = ?", result.PaymentID).
				Update("created_by", *createdBy).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Isi balik session
	session.ID = result.SessionID
	session.BookedDurationMinutes = result.SessionBookedMin
	session.Status = entity.SessionStatus(result.SessionStatus)
	session.TotalPrice = result.BaseAmount
	session.PaidDurationMinutes = result.PaidDurationMinutes
	session.TimeBankMinutesUsed = result.TimeBankMinutesUsed
	session.BonusMinutesFromPoints = result.BonusMinutesFromPoints
	session.PointsRedeemed = result.PointsRedeemed
	session.PointsEarned = result.PointsEarned

	// Bangun payment entity dari hasil SP
	payment := &entity.Payment{
		ID:                 result.PaymentID,
		SessionID:          result.SessionID,
		Amount:             result.BaseAmount,
		DiscountAmount:     result.DiscountAmount,
		AutoDiscountAmount: result.AutoDiscountAmount,
		TotalPayment:       result.TotalPayment,
		CashReceived:       result.CashReceived,
		ChangeAmount:       result.ChangeAmount,
		VoucherID:          result.VoucherID,
		PaymentMethod:      entity.PaymentMethod(paymentmethod.Normalize(paymentMethod)),
		PaymentStatus:      entity.PaymentStatusPaid,
	}
	return payment, nil
}

// AddSetupGrace menggeser waktu MULAI sesi (dan jadwal selesainya) sebesar
// beberapa menit untuk memberi waktu persiapan konsol. Karena start_time dan
// end_scheduled_at sama-sama digeser, durasi berbayar tidak berkurang dan
// harga/menit berbayar (paid_duration_minutes) tidak berubah. Waktu setup
// berada SEBELUM jam sesi berjalan, bukan tambahan di akhir sesi.
func (r *sessionRepository) AddSetupGrace(ctx context.Context, id uuid.UUID, minutes int) (*entity.Session, error) {
	var updated *entity.Session
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Row lock mencegah ExtendSession yang berjalan bersamaan tertimpa
		// oleh perhitungan start/end yang sudah basi.
		var session entity.Session
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&session, "id = ?", id).Error; err != nil {
			return err
		}
		if !session.ApplySetupGrace(minutes) {
			return nil
		}
		if err := tx.Model(&entity.Session{}).
			Where("id = ?", id).
			Updates(map[string]interface{}{
				"start_time":       session.StartTime,
				"end_scheduled_at": *session.EndScheduledAt,
				"updated_at":       time.Now(),
			}).Error; err != nil {
			return err
		}
		updated = &session
		return nil
	})
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return r.FindByID(ctx, id)
	}
	return r.FindByID(ctx, id)
}

// Update menggunakan byoneEndSession. Penutupan sesi dan reset konsol
// dijalankan dalam satu transaksi agar tidak ada status yang setengah jadi.
func (r *sessionRepository) Update(ctx context.Context, session *entity.Session) error {
	type endResult struct {
		TimeBankEarnedMinutes int `gorm:"column:time_bank_earned_minutes"`
	}
	var res endResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Raw(
			"SELECT * FROM "+spname.Ident("EndSession")+"(?)", session.ID,
		).Scan(&res).Error; err != nil {
			return parseStoredProcError(err)
		}
		return tx.Model(&entity.Console{}).
			Where("id = ?", session.ConsoleID).
			Updates(map[string]interface{}{
				"status":        entity.ConsoleStatusAvailable,
				"screen_status": entity.ScreenStatusOff,
				"manual_live":   false,
				"updated_at":    time.Now(),
			}).Error
	})
	if err != nil {
		return err
	}
	// Refresh data sesi setelah update
	if err := r.db.WithContext(ctx).First(session, "id = ?", session.ID).Error; err != nil {
		return err
	}
	session.TimeBankEarnedMinutes = res.TimeBankEarnedMinutes
	return nil
}

// UpdateStatus menggunakan GORM langsung (fallback untuk cancel)
func (r *sessionRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status entity.SessionStatus) error {
	if status == entity.SessionStatusCancelled {
		return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var consoleID uuid.UUID
			if err := tx.Table("sessions").Select("console_id").
				Where("id = ?", id).Scan(&consoleID).Error; err != nil {
				return err
			}
			if err := tx.Exec(
				fmt.Sprintf("SELECT %s(?)", spname.Ident("CancelSession")), id,
			).Error; err != nil {
				return parseStoredProcError(err)
			}
			if consoleID == uuid.Nil {
				return nil
			}
			return tx.Model(&entity.Console{}).
				Where("id = ?", consoleID).
				Updates(map[string]interface{}{
					"status":        entity.ConsoleStatusAvailable,
					"screen_status": entity.ScreenStatusOff,
					"manual_live":   false,
					"updated_at":    time.Now(),
				}).Error
		})
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
		"INVALID_DURATION",
		"NOT_A_MEMBER", "INSUFFICIENT_TIME_BANK", "INSUFFICIENT_POINTS",
		"VOUCHER_NOT_FOUND", "VOUCHER_INACTIVE", "VOUCHER_EXPIRED",
		"VOUCHER_LIMIT_REACHED", "VOUCHER_MIN_PURCHASE",
		"ORDER_EMPTY", "ORDER_NOT_FOUND", "ORDER_CANCELLED", "ORDER_SERVED",
		"INVALID_QUANTITY", "MENU_NOT_FOUND", "MENU_UNAVAILABLE", "INSUFFICIENT_STOCK",
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
