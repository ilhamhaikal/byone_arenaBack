package database

import (
	"fmt"
	"time"

	"byone-arena/pkg/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewGormDB membuat koneksi GORM ke PostgreSQL dengan timezone WIB
func NewGormDB(cfg *config.Config) (*gorm.DB, error) {
	logLevel := logger.Info
	if cfg.AppEnv == "production" {
		logLevel = logger.Error
	}

	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logLevel),
		PrepareStmt:                              true,
		DisableForeignKeyConstraintWhenMigrating: false,
	})
	if err != nil {
		return nil, fmt.Errorf("gagal terhubung ke database: %w", err)
	}

	// Set timezone WIB permanen di level database (berlaku untuk semua koneksi)
	db.Exec("ALTER DATABASE " + cfg.DBName + " SET timezone TO 'Asia/Jakarta'")

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("gagal mendapatkan sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(1 * time.Hour)
	sqlDB.SetConnMaxIdleTime(30 * time.Minute)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("gagal ping database: %w", err)
	}

	return db, nil
}
