# ==========================================
# Byone Arena - Makefile
# Kompatibel: Windows (PowerShell/Git Bash) & Linux/macOS
# ==========================================

.PHONY: help build run dev migrate db-shell tidy test test-coverage setup clean lint

# Variabel
BINARY_NAME=byone-arena
MAIN_PATH=./cmd/server

## help: Tampilkan bantuan perintah
help:
	@echo "============================================"
	@echo "  Byone Arena - PS Rental Management System"
	@echo "============================================"
	@echo "Perintah yang tersedia:"
	@echo ""
	@echo "  Development:"
	@echo "    make run        - Jalankan server langsung"
	@echo "    make dev        - Jalankan dengan hot-reload (butuh Air)"
	@echo "    make build      - Build binary"
	@echo "    make tidy       - Download & rapikan dependencies"
	@echo "    make test       - Jalankan semua test"
	@echo ""
	@echo "  Database:"
	@echo "    make migrate    - Jalankan migration database"
	@echo "    make db-shell   - Masuk ke PostgreSQL shell"
	@echo ""
	@echo "  Lainnya:"
	@echo "    make clean      - Hapus file build"
	@echo "    make setup      - Setup awal project (copy .env, download deps)"

## setup: Persiapan awal project
setup:
	@echo ">> Menyalin .env.example ke .env..."
	@cp -n .env.example .env || echo "   .env sudah ada, dilewati"
	@echo ">> Mengunduh dependencies..."
	@go mod download
	@echo ">> Setup selesai! Edit .env sesuai konfigurasi lokal Anda."

## tidy: Download dan rapikan dependencies
tidy:
	@echo ">> Menjalankan go mod tidy..."
	go mod tidy
	go mod verify

## build: Build binary
build:
	@echo ">> Membuild binary..."
	go build -ldflags="-w -s" -o $(BINARY_NAME) $(MAIN_PATH)
	@echo ">> Build selesai: $(BINARY_NAME)"

## run: Jalankan server secara langsung
run:
	@echo ">> Menjalankan server..."
	go run $(MAIN_PATH)

## dev: Jalankan dengan hot-reload menggunakan Air
dev:
	@which air > /dev/null 2>&1 || (echo "Air tidak ditemukan. Install dengan: go install github.com/air-verse/air@latest" && exit 1)
	@echo ">> Menjalankan server dengan hot-reload..."
	air -c .air.toml

## test: Jalankan semua test
test:
	@echo ">> Menjalankan test..."
	go test -v -race -cover ./...

## test-coverage: Jalankan test dengan laporan coverage
test-coverage:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo ">> Laporan coverage tersimpan di coverage.html"

## migrate: Jalankan migration database otomatis (semua .up.sql yang belum dijalankan)
migrate:
	@echo ">> Menjalankan migrasi database..."
	@go run ./cmd/migrate/

## db-shell: Masuk ke shell PostgreSQL
db-shell:
	@set -a; [ -f .env ] && . ./.env; set +a; \
	PGPASSWORD="$${DB_PASSWORD}" psql -h "$${DB_HOST:-127.0.0.1}" -U "$${DB_USER:-postgres}" -d "$${DB_NAME:-byone_arena}"

## clean: Hapus file build
clean:
	@echo ">> Membersihkan file build..."
	@rm -f $(BINARY_NAME) $(BINARY_NAME).exe
	@rm -rf tmp/ coverage.out coverage.html
	@echo ">> Selesai."

## lint: Jalankan linter (butuh golangci-lint)
lint:
	@which golangci-lint > /dev/null 2>&1 || (echo "golangci-lint tidak ditemukan. Install dari: https://golangci-lint.run/usage/install/" && exit 1)
	golangci-lint run ./...
