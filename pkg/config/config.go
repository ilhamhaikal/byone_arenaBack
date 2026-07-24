package config

import (
	"fmt"
	"os"
	"regexp"
	"strconv"

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
	JWTSecret      string
	JWTExpireHours int
}

// Load membaca konfigurasi dari file .env dan environment variables
func Load() (*Config, error) {
	// Hanya load .env jika file tersedia (diabaikan di production/Docker)
	_ = godotenv.Load()

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

		JWTSecret:      getEnv("JWT_SECRET", "change-this-secret-in-production"),
		JWTExpireHours: getEnvInt("JWT_EXPIRE_HOURS", 24),
	}

	if !spPrefixPattern.MatchString(cfg.SPPrefix) {
		return nil, fmt.Errorf("SP_PREFIX tidak valid: %q (hanya boleh huruf/angka, harus diawali huruf)", cfg.SPPrefix)
	}

	return cfg, nil
}

// DSN mengembalikan string koneksi PostgreSQL
func (c *Config) DSN() string {
	return "host=" + c.DBHost +
		" port=" + c.DBPort +
		" user=" + c.DBUser +
		" password=" + c.DBPassword +
		" dbname=" + c.DBName +
		" sslmode=" + c.DBSSLMode
}

// PostgresDSN mengembalikan DSN dalam format URL
func (c *Config) PostgresDSN() string {
	return "postgres://" + c.DBUser + ":" + c.DBPassword +
		"@" + c.DBHost + ":" + c.DBPort + "/" + c.DBName +
		"?sslmode=" + c.DBSSLMode
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
