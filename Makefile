# ==========================================
# Byone Arena - Makefile
# Kompatibel: Windows (PowerShell/Git Bash) & Linux/macOS
# ==========================================

.PHONY: help build run dev docker-up docker-down docker-dev migrate tidy test clean

# Variabel
BINARY_NAME=byone-arena
MAIN_PATH=./cmd/server
DOCKER_COMPOSE=docker compose

## help: Tampilkan bantuan perintah
help:
	@echo "============================================"
	@echo "  Byone Arena - PS Rental Management System"
	@echo "============================================"
	@echo "Perintah yang tersedia:"
	@echo ""
	@echo "  Development:"
	@echo "    make run        - Jalankan server langsung (tanpa Docker)"
	@echo "    make dev        - Jalankan dengan hot-reload (butuh Air)"
	@echo "    make build      - Build binary"
	@echo "    make tidy       - Download & rapikan dependencies"
	@echo "    make test       - Jalankan semua test"
	@echo ""
	@echo "  Docker:"
	@echo "    make docker-up      - Jalankan semua service (production mode)"
	@echo "    make docker-down    - Hentikan semua service"
	@echo "    make docker-dev     - Jalankan dalam mode development"
	@echo "    make docker-build   - Build ulang image Docker"
	@echo "    make docker-logs    - Lihat log semua container"
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

## docker-up: Jalankan semua service dengan Docker
docker-up:
	@echo ">> Memulai service Docker..."
	$(DOCKER_COMPOSE) up -d
	@echo ">> Semua service berjalan."
	@echo "   API  : http://localhost:$${PORT:-8080}"
	@echo "   WS   : ws://localhost:$${PORT:-8080}/ws"

## docker-down: Hentikan semua service Docker
docker-down:
	@echo ">> Menghentikan service Docker..."
	$(DOCKER_COMPOSE) down

## docker-dev: Jalankan dalam mode development (dengan pgAdmin)
docker-dev:
	@echo ">> Memulai service Docker (mode development)..."
	$(DOCKER_COMPOSE) -f docker-compose.yml -f docker-compose.dev.yml up -d
	@echo ">> Service development berjalan."
	@echo "   API     : http://localhost:$${PORT:-8080}"
	@echo "   pgAdmin : http://localhost:5050"

## docker-build: Build ulang image Docker
docker-build:
	@echo ">> Build ulang image Docker..."
	$(DOCKER_COMPOSE) build --no-cache

## docker-logs: Tampilkan log semua container
docker-logs:
	$(DOCKER_COMPOSE) logs -f

## docker-ps: Tampilkan status container
docker-ps:
	$(DOCKER_COMPOSE) ps

## migrate: Jalankan migration database (via Docker)
migrate:
	@echo ">> Menjalankan migration database (Docker)..."
	@docker exec -i byone-arena-db psql -U $${DB_USER:-postgres} -d $${DB_NAME:-byone_arena} < migrations/000001_init_schema.up.sql
	@docker exec -i byone-arena-db psql -U $${DB_USER:-postgres} -d $${DB_NAME:-byone_arena} < migrations/000002_add_shifts_and_procedures.up.sql
	@echo ">> Migration selesai."

## migrate-local: Jalankan migration database langsung via psql (tanpa Docker)
migrate-local:
	@echo ">> Menjalankan migration database (lokal)..."
	psql -h $${DB_HOST:-localhost} -p $${DB_PORT:-5432} -U $${DB_USER:-postgres} -d $${DB_NAME:-byone_arena} -f migrations/000001_init_schema.up.sql
	psql -h $${DB_HOST:-localhost} -p $${DB_PORT:-5432} -U $${DB_USER:-postgres} -d $${DB_NAME:-byone_arena} -f migrations/000002_add_shifts_and_procedures.up.sql
	@echo ">> Migration selesai."

## db-shell: Masuk ke shell PostgreSQL
db-shell:
	docker exec -it byone-arena-db psql -U $${DB_USER:-postgres} -d $${DB_NAME:-byone_arena}

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
