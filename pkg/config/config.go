package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

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

		JWTSecret:      getEnv("JWT_SECRET", "change-this-secret-in-production"),
		JWTExpireHours: getEnvInt("JWT_EXPIRE_HOURS", 24),
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
