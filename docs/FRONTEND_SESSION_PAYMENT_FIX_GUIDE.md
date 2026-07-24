# Panduan Adaptasi Frontend — Perbaikan Bug Kritikal Sesi & Pembayaran

> **Status backend**: sudah diperbaiki & live di database (migrasi `000050`, `000051`).
> **Status frontend**: **BELUM diperbaiki** — dokumen ini menjelaskan apa yang harus diubah.

---

## 1. Ringkasan Bug yang Sudah Diperbaiki di Backend

| # | Fungsi | Bug Lama | Fix |
|---|--------|----------|-----|
| 1 | `ExtendSession` (SP) | Waktu akhir sesi baru dihitung dari `NOW() + total_menit_kumulatif`, bukan dari waktu akhir lama + menit tambahan. Sisa waktu bisa naik/turun tidak masuk akal setiap kali "Tambah Sesi". | Waktu akhir baru = `waktu_akhir_lama_atau_sekarang + menit_tambahan_saja`. |
| 2 | `EndSession` (SP) | `total_price` dihitung ULANG dari nol berdasarkan `durasi_terpakai_aktual × harga_per_jam`, mengabaikan total yang sudah benar terakumulasi dari perpanjangan (extend) yang sudah dibayar. Sesi dengan 2× perpanjangan 30 menit @ Rp4.000 (harusnya total Rp8.000+) bisa berakhir dengan tagihan hanya Rp4.000 kalau diakhiri sebelum waktu extend habis terpakai. | Untuk sesi yang punya `bookedDurationMinutes` (pernah di-booking/di-extend), `total_price` diambil dari akumulasi yang sudah benar, bukan dihitung ulang dari waktu pakai aktual. |

**Kesimpulan penting**: mulai sekarang, field `totalPrice` pada response `PATCH /sessions/{id}/end` **SUDAH BENAR** dan mencerminkan seluruh biaya sesi (base + semua perpanjangan). Frontend **TIDAK BOLEH** menghitung ulang total tagihan sendiri.

---

## 2. Model Pembayaran per Sesi (PENTING — ini akar masalah di sisi frontend)

Satu sesi (`sessions`) bisa punya **lebih dari satu baris `payments`**:

- Setiap kali sesi di-**extend** (`POST /sessions/{id}/extend`), backend otomatis membuat **1 baris payment baru** untuk selisih harga (bisa `paid` langsung atau `pending` menunggu konfirmasi kasir).
- Payment perpanjangan **pertama** sebenarnya sudah mencakup harga dasar sesi + perpanjangan pertama (karena dihitung dari `total_price` lama yang masih 0).
- Payment perpanjangan **kedua dan seterusnya** hanya menagih selisihnya saja.
- Kalau sesi **tidak pernah** di-extend, tidak ada baris payment sama sekali sampai sesi diakhiri — baru dibuat 1 payment lewat `POST /payments` (endpoint "Bayar" di layar akhir sesi).

Artinya: **kalau sebuah sesi sudah pernah di-extend minimal 1×, seluruh tagihannya SUDAH tercakup oleh baris-baris payment perpanjangan tersebut.** Tidak perlu (dan TIDAK BOLEH) membuat payment baru lagi via `POST /payments` untuk sesi seperti itu.

Backend bahkan **menolak** hal ini secara eksplisit:
```
byoneCreatePayment akan RAISE EXCEPTION 'PAYMENT_EXISTS'
jika sesi sudah punya payment apa pun (selain yang berstatus refunded).
```

### Bug tersembunyi di frontend saat ini

File: `byone_arenaFront/lib/screens/rental/end_session_dialog.dart`

```dart
double get _cost => (_elapsed.inSeconds / 3600.0) * widget.session.pricePerHour;
```

`estimatedCost` dihitung sendiri oleh frontend dari waktu berjalan real-time × harga per jam. Ini **mengabaikan**:
- Perpanjangan yang sudah dibayar,
- Tier harga (`pricingTiers`) konsol,
- Diskon voucher.

Lalu di `_finish()`:
```dart
final endedSession = await sessionProvider.end(widget.session.id);
...
final payment = await paymentProvider.createCash(
  sessionId: widget.session.id,
  cashReceived: _cashReceived,
);
...
if (payment != null) {
  _showReceipt(payment.amount, payment.cashReceived, payment.changeAmount);
} else {
  // Session ended but payment recording failed — still show receipt
  _showReceipt(widget.estimatedCost, _cashReceived, _change);
}
```

Untuk **sesi yang pernah di-extend**, panggilan `createCash` (→ `POST /payments`) ini **AKAN SELALU GAGAL** dengan error `PAYMENT_EXISTS` (karena sudah ada payment dari extend). Kode di atas menangkap kegagalan itu secara diam-diam dan menampilkan struk memakai `estimatedCost` — angka yang dihitung sendiri oleh frontend dan **tidak mencerminkan total pembayaran yang sebenarnya**. Inilah penyebab langsung laporan bug "2 pembayaran pending 4.000 masing-masing, tapi struk akhir cuma menampilkan 4.000, padahal seharusnya 8.000".

---

## 3. Endpoint Backend yang Tersedia untuk Frontend

### 3.1 Endpoint BARU: ringkasan semua payment per sesi
```
GET /api/v1/sessions/{session_id}/payments
```
**Auth**: Bearer token
**Response**:
```json
{
  "data": {
    "payments": [ { "id": "...", "amount": 4000, "totalPayment": 4000, "paymentStatus": "paid", "createdAt": "..." }, ... ],
    "totalAmount": 8000,
    "totalPaid": 8000,
    "totalPending": 0
  }
}
```
Gunakan endpoint ini untuk menampilkan rincian & total pembayaran sesi yang sebenarnya — **jangan hitung sendiri di frontend.**

### 3.2 Endpoint existing yang relevan
- `PATCH /api/v1/sessions/{id}/end` → response sekarang membawa `totalPrice` yang **sudah benar** (akumulasi base + semua extend).
- `POST /api/v1/payments` (`byoneCreatePayment`) → **hanya valid untuk sesi yang BELUM PERNAH punya payment sama sekali** (sesi tanpa extend).
- `POST /api/v1/payments/{id}/confirm` → konfirmasi payment extend yang masih `pending` menjadi `paid`.
- `GET /api/v1/payments/pending` → daftar semua payment extend yang masih pending (dipakai admin/notifikasi, sudah benar, tidak perlu diubah).

---

## 4. Perubahan yang WAJIB Dilakukan di Frontend

### 4.1 `end_session_dialog.dart` — jangan hitung total sendiri, cabang alur sesuai riwayat payment

Alur yang benar:

1. Sebelum menampilkan dialog "Selesaikan Sesi", panggil `GET /sessions/{id}/payments`.
2. **Jika `payments` kosong** (sesi tidak pernah di-extend):
   - Tampilkan estimasi (boleh tetap dari elapsed time untuk preview), tapi setelah `end()` dipanggil, pakai `totalPrice` dari response `end()` sebagai total tagihan final — BUKAN `estimatedCost` frontend.
   - Lanjutkan `POST /payments` (createCash) seperti biasa untuk membayar `totalPrice` tersebut.
3. **Jika `payments` tidak kosong** (sesi pernah di-extend):
   - Kalau ada yang `pending`, wajib dikonfirmasi dulu (`POST /payments/{id}/confirm`) sebelum/saat mengakhiri sesi — tidak boleh dibiarkan pending setelah sesi selesai.
   - Setelah `end()` dipanggil, **JANGAN panggil `POST /payments` lagi** (akan gagal `PAYMENT_EXISTS`).
   - Tampilkan struk dari `totalAmount`/`totalPaid` hasil `GET /sessions/{id}/payments` (atau dari `totalPrice` pada response `end()`, keduanya sekarang seharusnya konsisten).

### 4.2 `payment_service.dart` — tambahkan method baru
```dart
Future<Map<String, dynamic>> getAllBySession(String sessionId) async {
  final response = await _api.get('${ApiConfig.sessions}/$sessionId/payments');
  return response['data'] as Map<String, dynamic>;
  // berisi: payments (List), totalAmount, totalPaid, totalPending
}
```

### 4.3 `payment_provider.dart` — tambahkan wrapper
```dart
Future<Map<String, dynamic>?> getAllBySession(String sessionId) async {
  try {
    return await _service.getAllBySession(sessionId);
  } catch (e) {
    _error = e.toString().replaceFirst('Exception: ', '');
    notifyListeners();
    return null;
  }
}
```

### 4.4 Jangan tampilkan struk memakai `estimatedCost` sebagai fallback

Hapus/ubah bagian ini di `_finish()`:
```dart
} else {
  // Session ended but payment recording failed — still show receipt
  _showReceipt(widget.estimatedCost, _cashReceived, _change);
}
```
Fallback diam-diam seperti ini yang menyembunyikan bug — kalau `createCash` gagal karena `PAYMENT_EXISTS`, itu adalah kondisi NORMAL untuk sesi yang di-extend (lihat 4.1), bukan error yang boleh ditutup-tutupi dengan angka perkiraan.

---

## 5. Ringkasan Perbaikan Backend yang Sudah Ditambahkan

| Perubahan | Lokasi |
|-----------|--------|
| Fix `ExtendSession` (waktu akhir sesi) | `migrations/000002_procedures.up.sql`, `migrations/000050_fix_extend_session_end_time.up.sql` |
| Fix `EndSession` (total_price) | `migrations/000002_procedures.up.sql`, `migrations/000051_fix_end_session_total_price.up.sql` |
| Endpoint baru `GET /sessions/{session_id}/payments` | `internal/delivery/http/handler/payment_handler.go` (`GetAllBySession`), `internal/delivery/http/router/router.go`, `internal/usecase/payment_usecase.go`, `internal/repository/postgres/payment_repository.go` |

Semua perubahan backend di atas sudah live di database development dan sudah lolos `go build ./...`.

---

## 6. Catatan Riwayat Data

Sesi yang **sudah selesai (`status = completed`) sebelum migrasi `000051`** kemungkinan punya `total_price` yang lebih kecil dari seharusnya (undercharge) untuk sesi yang pernah di-extend dan diakhiri sebelum seluruh waktu extend terpakai. Nilai ini **tidak bisa direkonstruksi otomatis** karena data historisnya sudah tertimpa — perlu pengecekan manual bila dibutuhkan untuk rekonsiliasi keuangan.
