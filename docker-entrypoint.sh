#!/bin/bash
# Docker entrypoint — gabungkan semua migration SQL untuk init PostgreSQL
# File ini dijalankan sebelum API server start

set -e

echo "🔧 Menjalankan migrasi database..."
PGPASSWORD="${DB_PASSWORD:-byone_secret}" psql \
    -h "${DB_HOST:-postgres}" \
    -U "${DB_USER:-postgres}" \
    -d "${DB_NAME:-byone_arena}" \
    -f /migrations/000001_init_schema.up.sql

PGPASSWORD="${DB_PASSWORD:-byone_secret}" psql \
    -h "${DB_HOST:-postgres}" \
    -U "${DB_USER:-postgres}" \
    -d "${DB_NAME:-byone_arena}" \
    -f /migrations/000002_procedures.up.sql

PGPASSWORD="${DB_PASSWORD:-byone_secret}" psql \
    -h "${DB_HOST:-postgres}" \
    -U "${DB_USER:-postgres}" \
    -d "${DB_NAME:-byone_arena}" \
    -f /migrations/000003_seed_data.up.sql

PGPASSWORD="${DB_PASSWORD:-byone_secret}" psql \
    -h "${DB_HOST:-postgres}" \
    -U "${DB_USER:-postgres}" \
    -d "${DB_NAME:-byone_arena}" \
    -f /migrations/000004_cleanup.up.sql

echo "✅ Migrasi selesai"
echo "🚀 Menjalankan API server..."
exec ./byone-arena
