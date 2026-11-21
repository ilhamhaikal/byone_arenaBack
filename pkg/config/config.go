package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// spPrefixPattern membatasi SP_PREFIX hanya huruf/angka dan diawali huruf,
// supaya aman disisipkan langsung ke nama fungsi/identifier SQL.
var spPrefixPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*$`)

// Config menyimpan semua konfigurasi aplikasi yang dimuat dari environment variables
type Config struct {
	// Server
	AppName  string
	AppEnv   string
	Port     string
	LogLevel string

	// Database
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	// White-label: prefix nama stored procedure/function di database (default "byone").
	// Bisa diganti per-client via env SP_PREFIX, misal "acme" -> acmeStartSession dst.
	SPPrefix string

	// JWT
	JWTSecret string
	// JWTSecretSource: env | file | generated | ephemeral (untuk logging).
	JWTSecretSource string
	JWTExpireHours  int

	// Direktori file update client (APK + manifest.json). Kosong = nonaktif.
	UpdatesDir string

	// Lisensi offline (Ed25519). Public key di-set vendor di .env.
	LicensePublicKey  string
	LicenseTrialDays  int
	LicenseTrialUnits int
	LicenseGraceDays  int

	// Discovery TV di LAN (UDP). TV broadcast probe, server membalas alamat API.
	DiscoveryEnabled bool
	DiscoveryPort    int
}

// Load membaca konfigurasi dari file .env dan environment variables
func Load() (*Config, error) {
	// Hanya load .env jika file tersedia (diabaikan di production/Docker)
	_ = godotenv.Load()

	jwtSecret, jwtSource := resolveJWTSecret()
	if jwtSecret == "" {
		return nil, fmt.Errorf("gagal menyiapkan JWT secret")
	}
	if isWeakJWTSecret(jwtSecret) {
		return nil, fmt.Errorf("JWT secret terlalu lemah: minimal 32 karakter dan cukup beragam")
	}

	cfg := &Config{
		AppName:  getEnv("APP_NAME", "Byone Arena"),
		AppEnv:   getEnv("APP_ENV", "development"),
		Port:     getEnv("PORT", "8080"),
		LogLevel: getEnv("LOG_LEVEL", "info"),

		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", ""),
		DBName:     getEnv("DB_NAME", "byone_arena"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		SPPrefix: getEnv("SP_PREFIX", "byone"),

		JWTSecret:       jwtSecret,
		JWTSecretSource: jwtSource,
		JWTExpireHours:  getEnvInt("JWT_EXPIRE_HOURS", 24),

		UpdatesDir: getEnv("UPDATES_DIR", ""),

		LicensePublicKey:  getEnv("LICENSE_PUBLIC_KEY", ""),
		LicenseTrialDays:  getEnvInt("LICENSE_TRIAL_DAYS", 14),
		LicenseTrialUnits: getEnvInt("LICENSE_TRIAL_UNITS", 2),
		LicenseGraceDays:  getEnvInt("LICENSE_GRACE_DAYS", 14),

		DiscoveryEnabled: getEnvBool("DISCOVERY_ENABLED", true),
		DiscoveryPort:    getEnvInt("DISCOVERY_PORT", 47823),
	}

	if !spPrefixPattern.MatchString(cfg.SPPrefix) {
		return nil, fmt.Errorf("SP_PREFIX tidak valid: %q (hanya boleh huruf/angka, harus diawali huruf)", cfg.SPPrefix)
	}

	return cfg, nil
}

// DSN mengembalikan string koneksi PostgreSQL
func (c *Config) DSN() string {
	// statement_cache_capacity=0 mematikan cache prepared statement di driver
	// pgx. Tanpa ini, migrasi yang menambah kolom (SELECT * berubah bentuk)
	// membuat query gagal dengan "cached plan must not change result type"
	// sampai aplikasi di-restart. Dengan cache dimatikan, perubahan skema
	// langsung berlaku.
	return "host=" + c.DBHost +
		" port=" + c.DBPort +
		" user=" + c.DBUser +
		" password=" + c.DBPassword +
		" dbname=" + c.DBName +
		" sslmode=" + c.DBSSLMode +
		" statement_cache_capacity=0" +
		" default_query_exec_mode=exec"
}

// PostgresDSN mengembalikan DSN dalam format URL
func (c *Config) PostgresDSN() string {
	return "postgres://" + c.DBUser + ":" + c.DBPassword +
		"@" + c.DBHost + ":" + c.DBPort + "/" + c.DBName +
		"?sslmode=" + c.DBSSLMode
}

// insecureJWTDefaults adalah nilai contoh yang tidak boleh dipakai.
var insecureJWTDefaults = map[string]bool{
	"change-this-secret-in-production":                                  true,
	"ganti-dengan-secret-key-yang-sangat-panjang-dan-aman-min-32-chars": true,
}

// isWeakJWTSecret menolak secret pendek atau terlalu monoton (mis. "aaaa...").
func isWeakJWTSecret(secret string) bool {
	secret = strings.TrimSpace(secret)
	if len(secret) < 32 {
		return true
	}
	unique := make(map[rune]struct{}, len(secret))
	for _, r := range secret {
		unique[r] = struct{}{}
	}
	return len(unique) < 10
}

// resolveJWTSecret menentukan JWT secret dengan urutan:
//  1. env JWT_SECRET (jika diisi dan bukan nilai contoh)
//  2. file JWT_SECRET_FILE / lokasi default
//  3. dibuat otomatis lalu disimpan
//
// Lokasi default: Windows -> %PROGRAMDATA%\ByoneArena\jwt.secret,
// platform lain -> ./jwt.secret. Dengan begitu klien tidak perlu mengisi
// JWT_SECRET di .env.
func resolveJWTSecret() (string, string) {
	if env := strings.TrimSpace(os.Getenv("JWT_SECRET")); env != "" && !insecureJWTDefaults[env] {
		return env, "env"
	}

	path := strings.TrimSpace(os.Getenv("JWT_SECRET_FILE"))
	if path == "" {
		if runtime.GOOS == "windows" {
			base := strings.TrimSpace(os.Getenv("PROGRAMDATA"))
			if base == "" {
				base = `C:\ProgramData`
			}
			path = filepath.Join(base, "ByoneArena", "jwt.secret")
		} else {
			path = "jwt.secret"
		}
	}

	if b, err := os.ReadFile(path); err == nil {
		if s := strings.TrimSpace(string(b)); s != "" {
			return s, "file"
		}
	}

	buf := make([]byte, 48)
	if _, err := rand.Read(buf); err != nil {
		return "", ""
	}
	secret := base64.RawURLEncoding.EncodeToString(buf)

	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return secret, "ephemeral"
		}
	}
	if err := os.WriteFile(path, []byte(secret+"\n"), 0o600); err != nil {
		return secret, "ephemeral"
	}
	return secret, "generated"
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		switch strings.ToLower(val) {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return defaultVal
}
