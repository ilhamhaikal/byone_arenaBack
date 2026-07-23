# =============================================
# Stage 1: Builder
# Menggunakan Go image untuk kompilasi
# =============================================
FROM golang:1.22-alpine AS builder

# Install dependensi build
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Copy go.mod dan go.sum terlebih dahulu untuk cache layer
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build binary dengan optimasi
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo 'dev')" \
    -o /app/byone-arena \
    ./cmd/server

# =============================================
# Stage 2: Runner
# Image minimal untuk production
# =============================================
FROM alpine:3.19 AS runner

# Install runtime dependencies (termasuk psql untuk migrasi)
RUN apk add --no-cache ca-certificates tzdata curl postgresql-client

# Set timezone ke WIB (Asia/Jakarta)
ENV TZ=Asia/Jakarta

# Buat user non-root untuk keamanan
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

WORKDIR /app

# Copy binary dari builder
COPY --from=builder /app/byone-arena .

# Copy migrations + entrypoint
COPY --from=builder /app/migrations ./migrations
COPY --from=builder /app/docker-entrypoint.sh .

# Ubah kepemilikan file
RUN chown -R appuser:appgroup /app && chmod +x docker-entrypoint.sh

# Switch ke user non-root
USER appuser

EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

ENTRYPOINT ["./docker-entrypoint.sh"]
