# Lisensi Offline Byone Arena

Sistem lisensi **offline**: server klien memverifikasi lisensi sendiri memakai
**public key** yang ditanam di binary. Tidak perlu server/domain vendor dan
tidak perlu internet.

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
| `expired` | Lewat masa tenggang | Read-only: sesi/pesanan/pairing baru diblokir |
| `none` | Belum ada lisensi & uji coba habis | Read-only |
| `invalid` | Lisensi gagal diverifikasi | Read-only |

Yang **tetap diizinkan** saat `expired`: mengakhiri sesi berjalan, pembayaran,
melihat laporan, dan mengimpor lisensi baru.

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
