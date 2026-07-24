package main

// @title           Byone Arena API
// @version         1.0
// @description     API untuk sistem manajemen rental PlayStation Byone Arena. Mendukung realtime via WebSocket untuk aplikasi mobile dan TV Android.
// @termsOfService  http://swagger.io/terms/

// @contact.name   Byone Arena Support
// @contact.email  support@byone-arena.com

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @host      localhost:8080
// @BasePath  /

// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                 Masukkan token JWT dengan format: Bearer {token}

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"byone-arena/internal/delivery/http/handler"
	"byone-arena/internal/delivery/http/router"
	wsHub "byone-arena/internal/delivery/websocket"
	"byone-arena/internal/domain/entity"
	pgRepo "byone-arena/internal/repository/postgres"
	"byone-arena/internal/usecase"
	"byone-arena/pkg/config"
	"byone-arena/pkg/database"
	"byone-arena/pkg/logger"
	"byone-arena/pkg/spname"
	appValidator "byone-arena/pkg/validator"

	_ "byone-arena/docs" // swagger docs

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// cleanupStaleNotifications membersihkan notifikasi yang sudah tidak relevan saat server start
func cleanupStaleNotifications(db *gorm.DB, log *zap.Logger) {
	// 1. "Sesi Diperpanjang" — notifikasi transient, cleanup semua yang masih aktif
	result := db.Exec(`
		UPDATE tv_notifications SET is_active = false, updated_at = NOW()
		WHERE title = 'Sesi Diperpanjang' AND is_active = true
	`)
	if result.Error != nil {
		log.Warn("Gagal cleanup notifikasi Sesi Diperpanjang", zap.Error(result.Error))
	} else if result.RowsAffected > 0 {
		log.Info("Cleanup notifikasi stale", zap.Int64("sesi_diperpanjang", result.RowsAffected))
	}

	// 2. "Pembayaran Tertunda" untuk sesi yang sudah tidak aktif
	result2 := db.Exec(`
		UPDATE tv_notifications n SET is_active = false, updated_at = NOW()
		FROM sessions s
		WHERE n.title = 'Pembayaran Tertunda'
		  AND n.is_active = true
		  AND n.target_console_ids::jsonb @> to_jsonb(s.console_id::TEXT)
		  AND s.status != 'active'
	`)
	if result2.Error != nil {
		log.Warn("Gagal cleanup notifikasi Pembayaran Tertunda", zap.Error(result2.Error))
	} else if result2.RowsAffected > 0 {
		log.Info("Cleanup notifikasi pembayaran tertunda stale", zap.Int64("count", result2.RowsAffected))
	}
}

func main() {
	// Load konfigurasi
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Gagal load konfigurasi: %v\n", err)
		os.Exit(1)
	}

	// Inisialisasi logger
	log, err := logger.New(cfg.AppEnv, cfg.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Gagal inisialisasi logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	log.Info("Memulai Byone Arena API",
		zap.String("app", cfg.AppName),
		zap.String("env", cfg.AppEnv),
		zap.String("port", cfg.Port),
	)

	// Set prefix nama stored procedure (white-label per client, default "byone")
	spname.Init(cfg.SPPrefix)

	// Koneksi database
	db, err := database.NewGormDB(cfg)
	if err != nil {
		log.Fatal("Gagal terhubung ke database", zap.Error(err))
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	log.Info("Koneksi database berhasil")

	// Cleanup stale notifikasi saat startup
	cleanupStaleNotifications(db, log)

	// Inisialisasi validator
	validate := appValidator.New()

	// Inisialisasi WebSocket Hub
	hub := wsHub.NewHub(log)
	hub.SetDB(db)
	go hub.Run()
	go hub.StartAutoStop() // auto-stop sesi expired

	// Auto-start notification loop jika ada notifikasi loop yang aktif
	go func() {
		time.Sleep(2 * time.Second) // tunggu DB ready
		var count int64
		db.Model(&entity.TvNotification{}).Where("loop_enabled = ? AND is_active = ?", true, true).Count(&count)
		if count > 0 {
			hub.StartNotificationLoop()
			log.Info("Notification loop auto-started", zap.Int64("active_notifications", count))
		}
	}()

	log.Info("WebSocket Hub berjalan")

	// Inisialisasi repository (infrastructure layer)
	consoleRepo := pgRepo.NewConsoleRepository(db)
	sessionRepo := pgRepo.NewSessionRepository(db)
	customerRepo := pgRepo.NewCustomerRepository(db)
	paymentRepo := pgRepo.NewPaymentRepository(db)
	userRepo := pgRepo.NewUserRepository(db)
	shiftRepo := pgRepo.NewShiftRepository(db)
	voucherRepo := pgRepo.NewVoucherRepository(db)
	discountRuleRepo := pgRepo.NewDiscountRuleRepository(db)
	menuRepo := pgRepo.NewMenuItemRepository(db)
	foodOrderRepo := pgRepo.NewFoodOrderRepository(db)

	// Inisialisasi use case (business logic layer)
	consoleUC := usecase.NewConsoleUseCase(consoleRepo, sessionRepo)
	sessionUC := usecase.NewSessionUseCase(sessionRepo, consoleRepo)
	customerUC := usecase.NewCustomerUseCase(customerRepo)
	paymentUC := usecase.NewPaymentUseCase(paymentRepo, sessionRepo)
	authUC := usecase.NewAuthUseCase(userRepo, shiftRepo, cfg)
	shiftUC := usecase.NewShiftUseCase(shiftRepo, userRepo)
	voucherUC := usecase.NewVoucherUseCase(voucherRepo)
	discountRuleUC := usecase.NewDiscountRuleUseCase(discountRuleRepo)
	menuUC := usecase.NewMenuItemUseCase(menuRepo)
	foodOrderUC := usecase.NewFoodOrderUseCase(foodOrderRepo)

	// Inisialisasi handler (delivery layer)
	handlers := &router.Handlers{
		Auth:      handler.NewAuthHandler(authUC, validate),
		Console:   handler.NewConsoleHandler(consoleUC, validate, db),
		Session:   handler.NewSessionHandler(sessionUC, validate, hub, db),
		Customer:  handler.NewCustomerHandler(customerUC, validate, db),
		Payment:   handler.NewPaymentHandler(paymentUC, validate, hub, db),
		Shift:     handler.NewShiftHandler(shiftUC),
		Voucher:   handler.NewVoucherHandler(voucherUC, validate),
		Discount:  handler.NewDiscountHandler(discountRuleUC, validate),
		Menu:      handler.NewMenuItemHandler(menuUC, validate),
		FoodOrder: handler.NewFoodOrderHandler(foodOrderUC, validate),
		Dashboard: handler.NewDashboardHandler(paymentRepo),
		Report:    handler.NewReportHandler(paymentRepo),
		Notify:    handler.NewNotificationHandler(db, hub, consoleUC, validate),
		Rental:    handler.NewRentalHandler(db, validate),
		Settings:  handler.NewSettingsHandler(db, validate),
		Activity:  handler.NewActivityHandler(db),
		Hub:       hub,
	}

	// Inisialisasi Fiber app
	app := fiber.New(fiber.Config{
		AppName:               cfg.AppName,
		ReadTimeout:           30 * time.Second,
		WriteTimeout:          30 * time.Second,
		IdleTimeout:           120 * time.Second,
		DisableStartupMessage: false,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			log.Error("Unhandled error", zap.Error(err), zap.Int("status", code))
			return c.Status(code).JSON(fiber.Map{
				"success": false,
				"message": err.Error(),
			})
		},
	})

	// Setup routes
	router.Setup(app, handlers, cfg)

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		addr := ":" + cfg.Port
		log.Info("Server berjalan", zap.String("address", addr))
		if err := app.Listen(addr); err != nil {
			log.Fatal("Server gagal berjalan", zap.Error(err))
		}
	}()

	<-quit
	log.Info("Menerima sinyal shutdown, menghentikan server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(ctx); err != nil {
		log.Error("Gagal shutdown server dengan baik", zap.Error(err))
	}

	log.Info("Server berhasil dihentikan")
}
