package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"byone-arena/pkg/config"
	"byone-arena/pkg/database"
)

// Migration menyimpan record migrasi yang sudah dijalankan
type Migration struct {
	Version  string `gorm:"primaryKey;size:20"`
	Filename string `gorm:"not null;size:255"`
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Gagal load config: %v\n", err)
		os.Exit(1)
	}

	db, err := database.NewGormDB(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Gagal koneksi database: %v\n", err)
		os.Exit(1)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	// Buat tabel tracker jika belum ada
	if err := db.AutoMigrate(&Migration{}); err != nil {
		fmt.Fprintf(os.Stderr, "Gagal inisialisasi tabel migrations: %v\n", err)
		os.Exit(1)
	}

	// Cari folder migrations
	migrationsDir := "migrations"
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		// Coba dari working directory lain
		migrationsDir = filepath.Join("..", "migrations")
	}
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		// Coba absolute dari project root
		if cwd, err := os.Getwd(); err == nil {
			migrationsDir = filepath.Join(cwd, "migrations")
		}
	}

	fmt.Printf("📁 Folder migrasi: %s\n", migrationsDir)

	// Baca semua file .up.sql
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Gagal membaca folder migrasi: %v\n", err)
		os.Exit(1)
	}

	var upFiles []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".up.sql") {
			upFiles = append(upFiles, name)
		}
	}
	sort.Strings(upFiles)

	if len(upFiles) == 0 {
		fmt.Println("✅ Tidak ada file migrasi ditemukan.")
		return
	}

	// Ambil daftar migrasi yang sudah dijalankan
	var applied []Migration
	db.Order("version ASC").Find(&applied)
	appliedMap := make(map[string]bool, len(applied))
	for _, m := range applied {
		appliedMap[m.Version] = true
	}

	// Jalankan migrasi yang belum applied
	ran := 0
	skipped := 0

	for _, filename := range upFiles {
		version := strings.SplitN(filename, "_", 2)[0] // "000001" dari "000001_init_schema.up.sql"

		if appliedMap[version] {
			fmt.Printf("⏭️  %s — sudah dijalankan\n", filename)
			skipped++
			continue
		}

		filePath := filepath.Join(migrationsDir, filename)
		sql, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Gagal membaca %s: %v\n", filename, err)
			os.Exit(1)
		}

		fmt.Printf("▶️  Menjalankan %s ... ", filename)

		// Substitusi placeholder __SP__ dengan prefix stored procedure sesuai
		// konfigurasi client (SP_PREFIX di .env, default "byone"). Ini yang
		// membuat nama function/procedure bisa di-white-label per client tanpa
		// mengubah file migrasi.
		sqlText := strings.ReplaceAll(string(sql), "__SP__", cfg.SPPrefix)

		// Eksekusi multi-statement SQL via *sql.DB langsung (bukan via GORM prepared stmt)
		if _, err := sqlDB.Exec(sqlText); err != nil {
			fmt.Printf("❌ GAGAL\n   %v\n", err)
			fmt.Fprintf(os.Stderr, "⚠️  Migrasi BERHENTI di %s. Perbaiki error lalu jalankan ulang.\n", filename)
			os.Exit(1)
		}

		// Catat migrasi sebagai applied
		record := Migration{Version: version, Filename: filename}
		if err := db.Create(&record).Error; err != nil {
			fmt.Printf("❌ GAGAL mencatat\n   %v\n", err)
			fmt.Fprintf(os.Stderr, "⚠️  SQL sudah dijalankan tapi record migrasi gagal disimpan.\n")
			os.Exit(1)
		}

		fmt.Println("✅")
		ran++
	}

	fmt.Printf("\n━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("✅ Selesai: %d dijalankan, %d dilewati\n", ran, skipped)
}
