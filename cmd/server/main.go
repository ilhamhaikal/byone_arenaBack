package main

// @title           Byone Arena API
// @version         1.0
// @description     API untuk sistem manajemen rental PlayStation Byone Arena. Mendukung realtime via WebSocket untuk aplikasi mobile dan TV Android.
// @termsOfService  http://swagger.io/terms/

// @contact.name   Byone Arena Support
// @contact.email  support@byone-arena.com

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @host      localhost:8081   (dev; saat runtime di-override sesuai PORT di .env)
// @BasePath  /

// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                 Masukkan token JWT dengan format: Bearer {token}

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata" // zona waktu tetap tersedia saat build Windows tanpa tzdata sistem

	"byone-arena/internal/delivery/http/handler"
	"byone-arena/internal/delivery/http/router"
	wsHub "byone-arena/internal/delivery/websocket"
	"byone-arena/internal/discovery"
	"byone-arena/internal/domain/entity"
	"byone-arena/internal/janitor"
	"byone-arena/internal/license"
	pgRepo "byone-arena/internal/repository/postgres"
	"byone-arena/internal/usecase"
	"byone-arena/pkg/config"
	"byone-arena/pkg/database"
	"byone-arena/pkg/licensekey"
	"byone-arena/pkg/logger"
	"byone-arena/pkg/spname"
	appValidator "byone-arena/pkg/validator"

	docs "byone-arena/docs" // swagger docs

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// cleanupStaleNotifications membersihkan notifikasi yang sudah tidak relevan saat server start
func cleanupStaleNotifications(db *gorm.DB, log *zap.Logger) {
	// 1. "Sesi Diperpanjang" — notifikasi transient, cleanup semua yang masih aktif
	var r1 struct {
		Updated int64 `gorm:"column:updated"`
	}
	tx1 := db.Raw("SELECT * FROM "+spname.Ident("DeactivateTvNotificationByTitle")+"(?::text)",
		"Sesi Diperpanjang").Scan(&r1)
	if tx1.Error != nil {
		log.Warn("Gagal cleanup notifikasi Sesi Diperpanjang", zap.Error(tx1.Error))
	} else if r1.Updated > 0 {
		log.Info("Cleanup notifikasi stale", zap.Int64("sesi_diperpanjang", r1.Updated))
	}

	// 2. "Pembayaran Tertunda" untuk sesi yang sudah tidak aktif
	var r2 struct {
		Updated int64 `gorm:"column:updated"`
	}
	tx2 := db.Raw("SELECT * FROM " + spname.Ident("DeactivateStalePendingPaymentNotifications") + "()").Scan(&r2)
	if tx2.Error != nil {
		log.Warn("Gagal cleanup notifikasi Pembayaran Tertunda", zap.Error(tx2.Error))
	} else if r2.Updated > 0 {
		log.Info("Cleanup notifikasi pembayaran tertunda stale", zap.Int64("count", r2.Updated))
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

	log.Info("JWT secret siap", zap.String("source", cfg.JWTSecretSource))
	if cfg.JWTSecretSource == "ephemeral" {
		log.Warn("JWT secret tidak dapat disimpan ke disk; sesi login akan hilang saat server restart")
	}

	// Tolak kombinasi environment yang berbahaya (dev memakai DB produksi,
	// atau production dijalankan dari workspace development).
	if err := cfg.ValidateRuntime(); err != nil {
		log.Error("Konfigurasi environment DITOLAK", zap.Error(err))
		log.Sync()
		fmt.Fprintf(os.Stderr, "\n  %v\n\n", err)
		os.Exit(1)
	}

	log.Info("Memulai Byone Arena API",
		zap.String("app", cfg.AppName),
		zap.String("env", cfg.AppEnv),
		zap.String("port", cfg.Port),
		zap.String("db", cfg.DBName),
	)

	// Swagger: host mengikuti port aktif (dev 8081, production 8080)
	docs.SwaggerInfo.Host = "localhost:" + cfg.Port

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

	// ── Lisensi offline ───────────────────────────────────────────────────
	// Public key DITANAM di binary (pkg/licensekey). ENV hanya untuk override
	// (white-label/rotasi). Private key TIDAK ada di server.
	encodedPub := strings.TrimSpace(cfg.LicensePublicKey)
	pubFromEnv := encodedPub != ""
	if !pubFromEnv {
		encodedPub = licensekey.DefaultPublicKeyB64
	}
	var licensePub ed25519.PublicKey
	if pub, pubErr := licensekey.ParsePublicKey(encodedPub); pubErr != nil {
		log.Error("Public key lisensi tidak valid — aktivasi lisensi tidak akan berfungsi", zap.Error(pubErr))
	} else {
		licensePub = pub
		if pubFromEnv {
			log.Info("Lisensi: public key dari ENV (override)")
		} else {
			log.Info("Lisensi: public key bawaan (embedded)")
		}
	}
	licenseSvc := license.NewService(db, licensePub,
		cfg.LicenseTrialDays, cfg.LicenseTrialUnits, cfg.LicenseGraceDays)

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
	ingredientRepo := pgRepo.NewIngredientRepository(db)
	capitalRepo := pgRepo.NewCapitalRepository(db)
	rentalPackageRepo := pgRepo.NewRentalPackageRepository(db)

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
	ingredientUC := usecase.NewIngredientUseCase(ingredientRepo)
	capitalUC := usecase.NewCapitalUseCase(capitalRepo)
	rentalPackageUC := usecase.NewRentalPackageUseCase(rentalPackageRepo)
	userUC := usecase.NewUserUseCase(userRepo)

	// Inisialisasi handler (delivery layer)
	handlers := &router.Handlers{
		Auth:          handler.NewAuthHandler(authUC, validate, db),
		Console:       handler.NewConsoleHandler(consoleUC, validate, db),
		Session:       handler.NewSessionHandler(sessionUC, validate, hub, db, licenseSvc),
		Customer:      handler.NewCustomerHandler(customerUC, validate, db),
		Payment:       handler.NewPaymentHandler(paymentUC, validate, hub, db),
		Shift:         handler.NewShiftHandler(shiftUC, db),
		Voucher:       handler.NewVoucherHandler(voucherUC, validate),
		Discount:      handler.NewDiscountHandler(discountRuleUC, validate),
		Menu:          handler.NewMenuItemHandler(menuUC, validate),
		Ingredient:    handler.NewIngredientHandler(ingredientUC, validate),
		FoodOrder:     handler.NewFoodOrderHandler(foodOrderUC, validate, db, hub, licenseSvc),
		Capital:       handler.NewCapitalHandler(capitalUC, validate),
		RentalPackage: handler.NewRentalPackageHandler(rentalPackageUC, validate, licenseSvc),
		Dashboard:     handler.NewDashboardHandler(paymentRepo, db),
		Report:        handler.NewReportHandler(paymentRepo),
		Notify:        handler.NewNotificationHandler(db, hub, consoleUC, validate),
		Rental:        handler.NewRentalHandler(db, validate, licenseSvc),
		Settings:      handler.NewSettingsHandler(db, validate),
		Activity:      handler.NewActivityHandler(db),
		User:          handler.NewUserHandler(userUC, validate, db),
		Device:        handler.NewDeviceHandler(db, licenseSvc),
		Audit:         handler.NewAuditHandler(db),
		Promo:         handler.NewPromoHandler(db, validate),
		License:       handler.NewLicenseHandler(licenseSvc, validate),
		Hub:           hub,
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
	router.Setup(app, handlers, cfg, db)

	// Discovery TV di LAN: TV broadcast probe, server membalas alamat API.
	if cfg.DiscoveryEnabled {
		discoveryCtx, discoveryCancel := context.WithCancel(context.Background())
		defer discoveryCancel()
		if err := discovery.Start(discoveryCtx, cfg.DiscoveryPort, cfg.Port, cfg.AppName, "1.0.0"); err != nil {
			log.Warn("Discovery UDP gagal berjalan (pairing TV tetap bisa manual)", zap.Error(err))
		} else {
			log.Info("Discovery UDP aktif", zap.Int("port", cfg.DiscoveryPort))
		}
	}

	// Janitor: tutup sesi yang terlantar (TV mati / aplikasi tidak aktif).
	janitorCtx, janitorCancel := context.WithCancel(context.Background())
	defer janitorCancel()
	janitor.StartStaleSessionWatcher(janitorCtx, db, log)

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
	janitorCancel()
	log.Info("Menerima sinyal shutdown, menghentikan server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(ctx); err != nil {
		log.Error("Gagal shutdown server dengan baik", zap.Error(err))
	}

	log.Info("Server berhasil dihentikan")
}
