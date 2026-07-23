@echo off
chcp 65001 >nul
title Byone Arena - Setup

echo =============================================
echo    🎮 BYONE ARENA - SETUP DATABASE
echo =============================================
echo.

if not exist ".env" (
    echo [1/3] Membuat file .env dari .env.example...
    copy .env.example .env >nul
    echo.
    echo ⚠️  FILE .ENV SUDAH DIBUAT!
    echo    Silakan edit file .env dengan Notepad:
    echo    - DB_PASSWORD  = password PostgreSQL Anda
    echo    - JWT_SECRET   = kata kunci rahasia (bebas)
    echo.
    echo    Setelah diedit, TUTUP Notepad, lalu tekan ENTER di sini...
    start /wait notepad .env
    echo.
)

echo [2/3] Menjalankan setup database...
echo.
migrate.exe
if %ERRORLEVEL% NEQ 0 (
    echo.
    echo ❌ SETUP GAGAL! Cek error di atas.
    echo    Pastikan PostgreSQL sudah terinstall dan .env sudah benar.
    pause
    exit /b 1
)

echo.
echo [3/3] Setup selesai!
echo.
echo =============================================
echo    ✅ DATABASE SIAP PAKAI!
echo =============================================
echo.
echo    Sekarang jalankan:  byone-server.exe
echo    Lalu buka browser:  http://localhost:8080
echo.
echo    Username: admin
echo    Password: password
echo.
pause
