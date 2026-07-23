# 🎮 Panduan Install Byone Arena (Untuk Pemula)

> **Baca ini baik-baik.** Kalau bingung, tanya ke tim support.  
> Dokumentasi ini untuk **Windows 10/11**. Semua langkah pakai bahasa sederhana.

---

## 📦 Isi Folder

Setelah Anda terima folder `byone-arena`, isinya seperti ini:

```
📁 byone-arena/
├── 📁 migrations/           ← jangan diutak-atik
├── ⚙️  .env.example         ← file contoh konfigurasi
├── 🚀 byone-server.exe      ← PROGRAM UTAMA (double-click)
├── 🔧 migrate.exe           ← PROGRAM SETUP DATABASE (jalankan 1x)
└── 📄 PANDUAN_INSTALL.md    ← file yang sedang Anda baca
```

---

## 🔧 Yang Harus Diinstall (CUKUP 1X)

### 1. Install PostgreSQL (Database)

> **Apa itu?** Tempat menyimpan data rental, pelanggan, pembayaran, dll.

**Langkah-langkah:**

1. Download PostgreSQL dari:  
   👉 **https://www.enterprisedb.com/downloads/postgres-postgresql-downloads**  
   Pilih versi **16.x** untuk **Windows x86-64**

2. Buka file installer yang sudah di-download (`postgresql-16.x-windows-x64.exe`)

3. Klik **Next** terus, sampai muncul halaman **Password**:

   ```
   ┌─────────────────────────────────┐
   │  Password for database          │
   │  superuser (postgres):          │
   │  [________________]             │
   │                                 │
   │  ⚠️ INI PENTING! CATAT PASSWORD! │
   └─────────────────────────────────┘
   ```

   > **Isi password yang mudah diingat**, misalnya: `By0N3-4r3NA`  
   > ⚠️ **CATAT password ini!** Nanti dipakai lagi.

4. Klik **Next** terus sampai **Finish**.

5. **BUKA pgAdmin** (ada di Start Menu) → ini buat ngecek database sudah jalan.  
   Kalau bisa login, berarti PostgreSQL sudah berhasil.

---

### 2. Setup Aplikasi Byone Arena

#### A. Bikin Database Baru

1. Buka **pgAdmin 4** dari Start Menu

2. Di panel kiri, klik kanan **Databases** → **Create** → **Database...**

3. Isi:
   ```
   Database:  byone_arena
   ```
   Klik **Save**.

   > ✅ Selesai! Database sudah dibuat.

#### B. Konfigurasi File .env

1. Di folder `byone-arena`, cari file `.env.example`

2. **Copy** file itu dan **rename** jadi `.env`  
   (Caranya: klik kanan `.env.example` → Copy → Paste → rename jadi `.env`)

3. **Klik kanan `.env` → Open with → Notepad**

4. Isi seperti ini (sesuaikan password PostgreSQL Anda):

   ```env
   DB_HOST=localhost
   DB_PORT=5432
   DB_USER=postgres
   DB_PASSWORD=By0N3-4r3NA        ← GANTI dengan password PostgreSQL Anda!
   DB_NAME=byone_arena
   DB_SSLMODE=disable

   JWT_SECRET=byone-arena-rental-2026  ← bebas, untuk keamanan login
   ```

5. **Save** (Ctrl+S), tutup Notepad.

#### C. Jalankan Setup Database

1. **Double-click file `migrate.exe`**

2. Akan muncul jendela hitam (Command Prompt) yang jalan sebentar lalu tutup sendiri.

3. Kalau muncul tulisan **"✅ Selesai"**, berarti setup database BERHASIL.

   > Kalau muncul ERROR, cek lagi file `.env` — mungkin password PostgreSQL salah.

#### D. Jalankan Server

1. **Double-click file `start.bat`** (atau `byone-server.exe`)

2. Akan muncul jendela hitam dengan tulisan:
   ```
   🎮 BYONE ARENA
   Server berjalan di http://localhost:8080
   ```

3. **JANGAN DITUTUP!** Biarkan jendela itu tetap terbuka.  
   (Minimize saja kalau mengganggu.)

---

## 🔧 Biar Otomatis Jalan Saat PC Nyala (Tidak Perlu Klik-Klik Lagi)

Setiap PC di-restart, server harus dijalankan ulang.  
Supaya tidak repot, bikin shortcut supaya **otomatis jalan sendiri**:

1. **Klik kanan file `start.bat`** → pilih **Create Shortcut**

2. Tekan **Win + R** di keyboard, ketik:
   ```
   shell:startup
   ```
   Lalu **Enter**.

3. Akan terbuka folder. **Copy-paste shortcut** yang tadi dibuat ke folder ini.

4. ✅ **Selesai!** Sekarang setiap PC dinyalakan, server akan jalan otomatis.

> **Catatan:** `migrate.exe` **cuma perlu dijalankan 1x** di awal setup.  
> Tidak perlu dimasukkan ke Startup.

> **Kalau ingin menghentikan auto-start:**  
> Buka lagi `shell:startup` (Win+R → `shell:startup`), hapus shortcut-nya.

---

## 🌐 Buka Aplikasi

1. Buka browser (Chrome / Edge)

2. Ketik alamat: **`http://localhost:8080`**

3. Halaman login akan muncul.

4. Login dengan:
   ```
   Username: admin
   Password: password
   ```

5. ✅ **Selesai!** Aplikasi sudah bisa dipakai.

---

## 📱 Akses dari HP / Tablet (Opsional)

Kalau ingin buka aplikasi dari HP atau tablet di jaringan yang sama (WiFi yang sama):

1. Cari IP address PC Anda:
   - Buka Command Prompt (Win+R, ketik `cmd`, Enter)
   - Ketik `ipconfig`, cari **IPv4 Address** (contoh: `192.168.1.5`)

2. Dari HP/tablet, buka browser dan ketik:
   ```
   http://192.168.1.5:8080
   ```
   (ganti `192.168.1.5` dengan IP PC Anda)

---

## 🔄 Cara Menjalankan Ulang Setelah Restart PC

Setelah PC di-restart:

1. Pastikan PostgreSQL jalan (biasanya otomatis).  
   Cek: buka pgAdmin, kalau bisa login = OK.

2. **Double-click `byone-server.exe`** — biarkan jendela hitam terbuka.

3. Buka browser → `http://localhost:8080`

---

## ❓ Masalah Umum

| Masalah | Solusi |
|---|---|
| "Tidak bisa connect ke database" | Pastikan PostgreSQL jalan. Cek di pgAdmin. |
| migrate.exe ERROR | Cek file `.env`, pastikan password PostgreSQL benar |
| "Port 8080 sudah dipakai" | Edit `.env`, ganti `PORT=8080` jadi `PORT=3000` |
| Lupa password admin | Hubungi support |
| Jendela hitam hilang sendiri | Buka Command Prompt, drag `byone-server.exe` ke situ, Enter — biar kelihatan errornya |

---

## 📞 Butuh Bantuan?

Hubungi: **support@byone-arena.com**  
Atau WA: **[nomor support]**
