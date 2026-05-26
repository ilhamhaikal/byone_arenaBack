package database

import (
	"fmt"
	"time"

	"byone-arena/pkg/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewGormDB membuat koneksi GORM ke PostgreSQL
func NewGormDB(cfg *config.Config) (*gorm.DB, error) {
	logLevel := logger.Info
	if cfg.AppEnv == "production" {
		logLevel = logger.Error
	}

	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logLevel),
		PrepareStmt:                              true, // cache prepared statements
		DisableForeignKeyConstraintWhenMigrating: false,
	})
	if err != nil {
		return nil, fmt.Errorf("gagal terhubung ke database: %w", err)
	}

	// Konfigurasi connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("gagal mendapatkan sql.DB dari GORM: %w", err)
	}

	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(1 * time.Hour)
	sqlDB.SetConnMaxIdleTime(30 * time.Minute)

	// Verifikasi koneksi
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("gagal ping database: %w", err)
	}

	return db, nil
}
