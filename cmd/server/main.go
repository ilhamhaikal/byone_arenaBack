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
	pgRepo "byone-arena/internal/repository/postgres"
	"byone-arena/internal/usecase"
	"byone-arena/pkg/config"
	"byone-arena/pkg/database"
	"byone-arena/pkg/logger"
	appValidator "byone-arena/pkg/validator"

	_ "byone-arena/docs" // swagger docs

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

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

	// Koneksi database
	db, err := database.NewGormDB(cfg)
	if err != nil {
		log.Fatal("Gagal terhubung ke database", zap.Error(err))
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	log.Info("Koneksi database berhasil")

	// Inisialisasi validator
	validate := appValidator.New()

	// Inisialisasi WebSocket Hub
	hub := wsHub.NewHub(log)
	go hub.Run()
	log.Info("WebSocket Hub berjalan")

	// Inisialisasi repository (infrastructure layer)
	consoleRepo := pgRepo.NewConsoleRepository(db)
	sessionRepo := pgRepo.NewSessionRepository(db)
	customerRepo := pgRepo.NewCustomerRepository(db)
	paymentRepo := pgRepo.NewPaymentRepository(db)
	userRepo := pgRepo.NewUserRepository(db)
	shiftRepo := pgRepo.NewShiftRepository(db)
	voucherRepo := pgRepo.NewVoucherRepository(db)

	// Inisialisasi use case (business logic layer)
	consoleUC := usecase.NewConsoleUseCase(consoleRepo)
	sessionUC := usecase.NewSessionUseCase(sessionRepo, consoleRepo)
	customerUC := usecase.NewCustomerUseCase(customerRepo)
	paymentUC := usecase.NewPaymentUseCase(paymentRepo, sessionRepo)
	authUC := usecase.NewAuthUseCase(userRepo, shiftRepo, cfg)
	shiftUC := usecase.NewShiftUseCase(shiftRepo, userRepo)
	voucherUC := usecase.NewVoucherUseCase(voucherRepo)

	// Inisialisasi handler (delivery layer)
	handlers := &router.Handlers{
		Auth:     handler.NewAuthHandler(authUC, validate),
		Console:  handler.NewConsoleHandler(consoleUC, validate),
		Session:  handler.NewSessionHandler(sessionUC, validate, hub),
		Customer: handler.NewCustomerHandler(customerUC, validate),
		Payment:  handler.NewPaymentHandler(paymentUC, validate, hub),
		Shift:    handler.NewShiftHandler(shiftUC),
		Voucher:  handler.NewVoucherHandler(voucherUC, validate),
		Hub:      hub,
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
