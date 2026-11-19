#!/usr/bin/env bash
# =============================================================================
# Build paket Windows Byone Arena (server + database setup + installer).
#
# Kebutuhan:
#   - Go (untuk cross-compile)
#   - goversioninfo : go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest
#   - makensis      : apt-get install nsis
#
# Pemakaian:
#   ./scripts/build_windows.sh            # versi default 1.0.0
#   VERSION=1.0.5 ./scripts/build_windows.sh
#
# Hasil di dist/windows/:
#   byone-arena-setup-<versi>.exe
#   payload/ (isi mentah, untuk diagnosis)
#   SHA256SUMS.txt
# =============================================================================
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

VERSION="${VERSION:-1.0.0}"
if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "❌ VERSION harus format x.y.z (contoh 1.0.0)" >&2
  exit 1
fi

DIST="$ROOT/dist/windows"
PAYLOAD="$DIST/payload"
PKG="$ROOT/packaging/windows"

command -v go >/dev/null || { echo "❌ go tidak ditemukan" >&2; exit 1; }
command -v goversioninfo >/dev/null || {
  echo "❌ goversioninfo tidak ditemukan." >&2
  echo "   Install: go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest" >&2
  exit 1
}
command -v makensis >/dev/null || { echo "❌ makensis tidak ditemukan (apt-get install nsis)" >&2; exit 1; }

echo "▶️  Build Windows v$VERSION"
rm -rf "$DIST"
mkdir -p "$PAYLOAD"

# ── 1. Versi resource (version info) ────────────────────────────────────────
echo "  • menyiapkan version info"
python3 - "$PKG" "$VERSION" <<'PY'
import json, sys, pathlib
pkg, ver = pathlib.Path(sys.argv[1]), sys.argv[2]
parts = (ver.split(".") + ["0", "0", "0"])[:3]
major, minor, patch = (int(x) for x in parts)
# Manifest dengan versi assembly yang valid
manifest = (pkg / "app.manifest").read_text().replace("__VERSION__", f"{major}.{minor}.{patch}.0")
(pkg / "app.manifest.build").write_text(manifest)
for name in ("versioninfo-server.json", "versioninfo-migrate.json"):
    d = json.loads((pkg / name).read_text())
    d["FixedFileInfo"]["FileVersion"] = {"Major": major, "Minor": minor, "Patch": patch, "Build": 0}
    d["FixedFileInfo"]["ProductVersion"] = {"Major": major, "Minor": minor, "Patch": patch, "Build": 0}
    d["StringFileInfo"]["FileVersion"] = f"{major}.{minor}.{patch}.0"
    d["StringFileInfo"]["ProductVersion"] = ver
    d["ManifestPath"] = "app.manifest.build"
    (pkg / (name + ".build.json")).write_text(json.dumps(d, indent=2))
PY

( cd "$PKG" \
  && goversioninfo -64 -o "$ROOT/cmd/server/resource_windows_amd64.syso" versioninfo-server.json.build.json \
  && goversioninfo -64 -o "$ROOT/cmd/migrate/resource_windows_amd64.syso" versioninfo-migrate.json.build.json )
rm -f "$PKG"/*.build.json "$PKG"/app.manifest.build

# ── 2. Cross-compile ───────────────────────────────────────────────────────
echo "  • kompilasi server & migrate (windows/amd64)"
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags="-s -w" -o "$PAYLOAD/byone-server.exe" ./cmd/server
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags="-s -w" -o "$PAYLOAD/migrate.exe" ./cmd/migrate

# ── 3. Payload pendukung ───────────────────────────────────────────────────
echo "  • menyalin migrations & dokumentasi"
cp -a "$ROOT/migrations" "$PAYLOAD/migrations"
mkdir -p "$PAYLOAD/docs"
cat > "$PAYLOAD/.env.example" <<'ENV'
# ==========================================
# Byone Arena - Konfigurasi Server
# ==========================================
APP_NAME=Byone Arena
APP_ENV=production
PORT=8080
LOG_LEVEL=info

# ==========================================
# Database PostgreSQL (WAJIB DIISI)
# ==========================================
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=GANTI_DENGAN_PASSWORD_DATABASE
DB_NAME=byone_arena
DB_SSLMODE=disable

# ==========================================
# White-label (biarkan default)
# ==========================================
SP_PREFIX=byone

# ==========================================
# JWT (kosong = dibuat otomatis)
# ==========================================
JWT_SECRET=
JWT_EXPIRE_HOURS=24
ENV
cp -a "$ROOT/docs/PANDUAN_INSTALL_WINDOWS.md" "$PAYLOAD/docs/" 2>/dev/null || true
cp -a "$ROOT/docs/LISENSI.md" "$PAYLOAD/docs/" 2>/dev/null || true

cat > "$PAYLOAD/start-server.cmd" <<'CMD'
@echo off
rem Jalankan server dari folder instalasi agar .env & migrations ditemukan.
cd /d "%~dp0"
"%~dp0byone-server.exe"
CMD

cat > "$PAYLOAD/run-migrate.cmd" <<'CMD'
@echo off
cd /d "%~dp0"
echo Menjalankan migrasi database...
"%~dp0migrate.exe"
echo.
echo Selesai. Tekan tombol apa saja untuk menutup.
pause >nul
CMD

cat > "$PAYLOAD/README-WINDOWS.txt" <<'TXT'
BYONE ARENA — SERVER (WINDOWS)

Cukup ubah 2 baris:
1. Di folder ini, salin ".env.example" menjadi ".env".
2. Buka ".env" dengan Notepad.
3. Isi DB_USER dan DB_PASSWORD sesuai PostgreSQL Anda.
   (JWT secret & public key lisensi sudah otomatis.)
4. Jalankan "Setup Database" dari Start Menu (sekali saja).
5. Jalankan "Start Server", atau restart PC jika auto-start dipilih.

Port server default: 8080 (aturan firewall dibuat otomatis oleh installer).

Jika Windows menampilkan peringatan:
- "Windows protected your PC" -> More info -> Run anyway.
- File terkunci setelah diunduh -> klik kanan file -> Properties -> Unblock.
- Prompt firewall -> Allow access.

Lisensi: buka aplikasi admin -> menu Lisensi -> Impor File Lisensi.
TXT

# ── 4. Installer ───────────────────────────────────────────────────────────
echo "  • membuat installer NSIS"
OUTFILE="$DIST/byone-arena-setup-$VERSION.exe"
( cd "$PKG" && makensis -DPAYLOAD="$PAYLOAD" -DOUTFILE="$OUTFILE" -DVERSION="$VERSION" installer.nsi )

# ── 5. Checksum ────────────────────────────────────────────────────────────
echo "  • menghitung SHA-256"
( cd "$DIST" && sha256sum "byone-arena-setup-$VERSION.exe" "payload/byone-server.exe" "payload/migrate.exe" > SHA256SUMS.txt )

echo
echo "✅ Selesai"
echo "   Installer : $OUTFILE"
echo "   Ukuran    : $(du -h "$OUTFILE" | cut -f1)"
echo "   Checksum  : $DIST/SHA256SUMS.txt"
echo
cat "$DIST/SHA256SUMS.txt"
