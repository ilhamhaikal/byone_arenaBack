#!/usr/bin/env bash
# Menjalankan backend dalam MODE DEVELOPMENT.
# Selalu memakai DB dev (byone_arena) dan port 8081.
set -euo pipefail
cd "$(dirname "$0")/.."

export APP_ENV=development
export PORT="${PORT:-8081}"
export DB_NAME="${DB_NAME:-byone_arena}"

echo "=============================================="
echo " MODE DEVELOPMENT"
echo " env      : $APP_ENV"
echo " database : $DB_NAME"
echo " port     : $PORT"
echo "=============================================="

exec go run ./cmd/server
