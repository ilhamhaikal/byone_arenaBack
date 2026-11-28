# Lisensi Byone Arena (Offline + Aktivasi Online)

Sistem lisensi memakai **Ed25519** dan tetap bisa berjalan **offline** setelah
lisensi diterima. Public key ditanam di binary server klien, sehingga server
klien tidak dapat menerbitkan lisensi sendiri.

Ada dua jalur aktivasi:

1. **Aktivasi online QR (disarankan)** — server billing meminta kode ke license
   server vendor, menampilkan QR/kode, vendor menyetujui, lalu server billing
   menerima token bertanda tangan. Internet hanya dibutuhkan saat aktivasi dan
   heartbeat.
2. **Impor file/kunci manual** — fallback untuk instalasi tanpa akses langsung
   ke license server.

Heartbeat opsional memeriksa pencabutan/penangguhan lisensi. Jika heartbeat
gagal, operasional lokal tetap berjalan sampai expiry + grace, sesuai model
lisensi offline.

## Konsep

- Lisensi ditandatangani vendor dengan **Ed25519**.
- Server hanya memegang **public key** → tidak bisa membuat lisensi sendiri.
- Lisensi mengikat **ID server** (UUID yang dibuat saat instalasi) sehingga
  tidak bisa dipakai di server lain.
- Jumlah unit = **TV yang terhubung** ke konsol (`consoles.device_id`).
  TV yang di-install tapi belum dipasangkan tidak dihitung.
- `maxUnits = 0` berarti **tanpa batas unit** (enterprise), tetapi masa berlaku
  tetap ada.

## Status lisensi

| State | Arti | Perilaku |
|---|---|---|
| `trial` | Belum ada lisensi, masih masa uji coba | Operasional, maks unit trial |
| `active` | Aktif, sisa > 30 hari | Normal |
| `expiring` | Aktif, sisa ≤ 30 hari | Normal + banner peringatan |
| `grace` | Lewat expiry, masih masa tenggang | Operasional + banner |
| `expired` | Lewat masa tenggang | Read-only |
| `none` | Belum ada lisensi & uji coba habis | Read-only |
| `invalid` | Lisensi gagal diverifikasi | Read-only |
| `suspended` | Ditangguhkan vendor lewat heartbeat | Read-only + aktivasi |
| `revoked` | Dicabut vendor lewat heartbeat | Read-only + aktivasi |
| `inactive` | Aktivasi perangkat tidak aktif lagi | Read-only + aktivasi |

Yang **tetap diizinkan** saat read-only: mengakhiri sesi berjalan, menerima
pembayaran, menutup shift, melepas perangkat, melihat laporan, dan melakukan
aktivasi lisensi baru.

## Integrasi online

### Konfigurasi server billing

Tambahkan ke `.env` server billing:

```env
APP_VERSION=1.0.4
LICENSE_SERVER_URL=https://license.byonebilling.id
LICENSE_HTTP_TIMEOUT_SECONDS=10
LICENSE_HEARTBEAT_MINUTES=30
```

`LICENSE_SERVER_URL` kosong berarti integrasi online nonaktif dan hanya
aktivasi manual yang tersedia. Untuk development LAN boleh memakai
`http://<IP-vendor>:8090`; produksi wajib HTTPS (loopback/LAN privat tetap
diizinkan untuk deployment lokal).

### Alur aktivasi QR

1. Owner membuka **menu Lisensi → Aktivasi Online (QR)**.
2. Server billing memanggil `POST /api/v1/license/online/start`; server
   billing membuat request ke license server vendor dan menyimpan **secret
   terenkripsi** di database lokal. Secret tidak pernah dikirim ke Flutter.
3. Flutter menampilkan QR berisi kode `REQ-...`.
4. Vendor memindai kode di portal **Aktivasi perangkat** dan menerbitkan
   lisensi.
5. Flutter memanggil `GET /api/v1/license/online/status` setiap beberapa detik.
   Server billing yang melakukan polling ke vendor, memverifikasi signature,
   lalu menyimpan token.
6. Status lisensi berubah menjadi `active`; dashboard terbuka.

Endpoint aktivasi hanya untuk **superadmin/owner**. Token dan secret tidak
pernah masuk ke QR; QR hanya berisi kode request.

### Heartbeat dan revocation

- Heartbeat berjalan dari server billing ke vendor setiap
  `LICENSE_HEARTBEAT_MINUTES` (default 30 menit).
- Heartbeat mengirim token lisensi + instance ID, memperbarui `last_seen`,
  mengambil token segar (mis. setelah perpanjangan), dan menyimpan status
  remote (`active`/`suspended`/`revoked`/`expired`/`inactive`).
- State remote disimpan di database lokal dan tetap berlaku setelah restart.
- Kegagalan jaringan **tidak** memblokir operasional; lisensi tetap memakai
  expiry + grace. Revocation baru berlaku setelah heartbeat sukses.
- Aktivasi ulang melalui QR/manual dapat memulihkan instalasi yang dicabut.

### Gerbang lisensi (server-side)

- Middleware `LicenseGate` memblokir operasi tulis baru saat lisensi tidak
  mengizinkan, tetapi mengizinkan GET, login/logout, ganti password, pembayaran,
  penutupan sesi/shift, pelepasan perangkat, dan aktivasi lisensi.
- Endpoint publik TV (`/ws`, `/consoles/overview`, heartbeat konsol, pairing)
  memakai `RequireValidLicense`, sehingga TV tidak bisa pairing/memakai API saat
  server billing terkunci. Instalasi APK tetap tidak diblokir.
- Pemeriksaan juga tetap ada di handler bisnis (defense in depth), bukan hanya
  di UI Flutter.

## Arsitektur implementasi

| Komponen | Tanggung jawab |
|---|---|
| `internal/license/vendor_client.go` | HTTP client ke license server; timeout, batas body, redirect dikunci ke host yang sama, User-Agent versi aplikasi |
| `internal/license/online.go` | Mulai aktivasi QR, polling, enkripsi secret, heartbeat, penyimpanan state remote |
| `internal/license/secrets.go` | AES-GCM untuk secret aktivasi di database lokal |
| `internal/license/service.go` | Verifikasi Ed25519, status aktif/expired/grace, unit, remote state, deteksi clock rollback |
| `internal/delivery/http/middleware/license_middleware.go` | `LicenseGate` untuk operasi tulis dan `RequireValidLicense` untuk endpoint TV |
| `migrations/000062..000064` | Tabel request aktivasi, kolom heartbeat, SP aktivasi/heartbeat, dan `GetActiveLicenseV2` agar rolling deploy aman |

Alur data tidak bergantung pada Flutter untuk keamanan: Flutter hanya
menampilkan QR dan meminta server billing memulai/memeriksa aktivasi. Secret
vendor tetap di server billing, dan token lisensi diverifikasi ulang oleh
server billing sebelum disimpan.

### Deployment

1. Isi `LICENSE_SERVER_URL` (HTTPS produksi) dan `APP_VERSION` di `.env`.
2. Jalankan `go run ./cmd/migrate` (atau migrasi saat start).
3. Restart server billing.
4. Login sebagai owner → **Lisensi → Aktivasi Online (QR)**.
5. Setelah aktif, pantau `last_heartbeat_at`/`remote_state` di tabel `licenses`.

## Untuk vendor

### 1. Buat pasangan kunci (sekali saja)

```bash
go run ./cmd/licensegen keygen -out-dir ./license-keys
```

- `license-keys/private.key` → **RAHASIA**, simpan offline (USB/password
  manager), jangan masuk repo.
- `license-keys/public.key` → **tidak perlu ditempel ke `.env`**: public key
  sudah ditanam di binary server. Baris `LICENSE_PUBLIC_KEY` di `.env` hanya
  dipakai bila ingin override (white-label/rotasi kunci).

> Jika private key hilang, lisensi baru tidak bisa diterbitkan untuk instalasi
> lama. Wajib di-backup.

### 2. Minta ID Server dari klien

Klien membuka **menu Lisensi** di aplikasi admin, lalu menyalin **ID Server**
(tombol salin) dan mengirimkannya.

### 3. Terbitkan lisensi

```bash
go run ./cmd/licensegen issue \
  -customer "Warnet Budi" \
  -plan GROWTH \
  -units 5 \
  -days 365 \
  -instance <ID-SERVER-KLIEN> \
  -out budi.lic
```

Kirim `budi.lic` lewat WhatsApp/email.

### 4. Perpanjangan

Sama seperti di atas, dengan `-days 365` baru (atau `-expires YYYY-MM-DD`).
Klien mengimpor file baru; data tidak berubah.

### 5. Tambah unit (upgrade)

Naikkan `-units`, **pertahankan tanggal expiry** agar sisa waktu tidak hilang:

```bash
go run ./cmd/licensegen issue \
  -customer "Warnet Budi" -plan GROWTH \
  -units 8 -expires 2027-11-17 \
  -instance <ID-SERVER-KLIEN> \
  -parent BYN-20261119-CC2E \
  -out budi-upgrade.lic
```

Klien mengimpor file upgrade; lisensi lama otomatis digantikan, masa berlaku
tetap.

### 6. Memeriksa file lisensi

```bash
go run ./cmd/licensegen inspect -license budi.lic -public ./license-keys/public.key
```

## Untuk klien

1. Buka **menu Lisensi** → **Impor File Lisensi** → pilih `.lic`.
2. Muncul notifikasi **Lisensi Berhasil Diaktifkan** berisi rincian unit,
   masa berlaku, dan sisa hari.
3. Status dan peringatan masa berlaku tampil juga di **dashboard**.

## Keamanan

- Perubahan isi lisensi (mis. tanggal) merusak tanda tangan → ditolak.
- Mengedit tabel `licenses` di database **tidak** menaikkan unit/masa berlaku,
  karena server memakai nilai dari payload yang ditandatangani.
- Mengimpor lisensi yang menurunkan masa berlaku/unit akan meminta
  **konfirmasi** lebih dulu.
- Batas unit ditegakkan saat **memasangkan TV** (`bind`), bukan saat install
  APK.
- Karena aplikasi self-hosted, patch binary oleh pihak yang sangat berniat
  tetap mungkin; lisensi bertanda tangan mencegah **pemalsuan**, bukan
  pembongkaran binary. Ini batas wajar model offline.
