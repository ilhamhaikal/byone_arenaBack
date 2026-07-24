# Dokumentasi Integrasi — Pendapatan Makanan/Minuman/Snack di Dashboard & Laporan

> **Backend adalah source of truth. Frontend mengikuti backend.**
> Dokumen ini merangkum perubahan backend yang SUDAH SELESAI & TERVERIFIKASI, agar tim frontend
> (aplikasi manapun yang dipakai untuk render Dashboard & Laporan) bisa mengimplementasikan
> pemisahan kartu "Pendapatan Makanan" tanpa perlu membaca source Go/SQL secara langsung.

---

## 1. Latar Belakang

Sebelumnya pendapatan dari penjualan makanan/minuman/snack (via `food_orders` + `menus`)
tercampur/tidak muncul terpisah di Dashboard maupun Laporan. Backend sekarang menghitung
pendapatan makanan sebagai agregat **terpisah** dari pendapatan sewa konsol/membership, dan
mengirimkannya sebagai field tambahan pada 2 endpoint yang sudah ada (tidak ada endpoint baru).

- Migration: `migrations/000053_add_food_sales_to_reports.up.sql`
- Stored procedures yang diubah: `byoneDashboardSummary(date)`, `byoneReportSummary(startDate, endDate)`
- Sumber data: tabel `food_orders` (status `served`) join `menus` untuk kategori/nama item.
- Kriteria "terjual/revenue diakui": `food_orders.status = 'served'` (order yang sudah selesai
  disajikan), difilter berdasarkan tanggal `updated_at`/tanggal transaksi sesuai periode.

Tidak ada breaking change — field lama tetap ada dengan nama & tipe yang sama, hanya field baru
yang ditambahkan.

---

## 2. Dashboard — `GET /api/v1/dashboard/summary`

```
GET /api/v1/dashboard/summary?date=YYYY-MM-DD
```
**Auth**: Bearer token
**Query param**: `date` opsional (default: hari ini)

### Field baru pada response `data`

| Field | Tipe | Keterangan |
|---|---|---|
| `foodSalesRevenue` | number (float) | Total pendapatan makanan/minuman/snack pada tanggal tersebut |
| `foodOrderCount` | number (int) | Jumlah order makanan (`status = 'served'`) pada tanggal tersebut |
| `pendingFoodOrders` | number (int) | Jumlah order makanan yang masih pending/belum disajikan (real-time, tidak terikat filter tanggal) |

### Contoh response lengkap

```json
{
  "success": true,
  "message": "Ringkasan dashboard berhasil diambil",
  "data": {
    "date": "2026-07-24",
    "totalRevenue": 1250000,
    "totalBaseAmount": 1300000,
    "totalTransactions": 18,
    "totalDiscount": 50000,
    "totalAutoDiscount": 20000,
    "voucherUsageCount": 3,
    "totalCashReceived": 1250000,
    "totalChange": 5000,
    "dailyRentalRevenue": 400000,
    "dailyRentalCount": 2,
    "membershipRevenue": 300000,
    "membershipCount": 1,
    "foodSalesRevenue": 185000,
    "foodOrderCount": 12,
    "pendingFoodOrders": 2,
    "activeSessions": 3,
    "availableConsoles": 5,
    "totalConsoles": 8,
    "voucherDetails": [ /* ... */ ],
    "generatedAt": "2026-07-24T16:00:00Z"
  }
}
```

### Rekomendasi UI — Dashboard

Tambahkan **kartu baru terpisah** (jangan digabung ke kartu pendapatan sewa/total):

- Kartu "Pendapatan Makanan": tampilkan `foodSalesRevenue` (format `Rp #.###`) + subtitle
  `"{foodOrderCount} order"`.
- Kartu "Order Makanan Pending" (opsional, badge warning jika > 0): tampilkan `pendingFoodOrders`.
- `totalRevenue` **tetap** merepresentasikan total keseluruhan (sudah termasuk makanan di backend
  bila relevan) — kartu makanan ini hanya untuk *breakdown*/transparansi sumber pendapatan, bukan
  angka tambahan yang harus dijumlahkan manual di frontend.

---

## 3. Laporan — `GET /api/v1/reports/summary`

```
GET /api/v1/reports/summary?startDate=YYYY-MM-DD&endDate=YYYY-MM-DD
```
**Auth**: Bearer token
**Query params**: `startDate` (default: 7 hari lalu), `endDate` (default: hari ini)

### 3.1 Field baru di `data.revenue`

| Field | Tipe | Keterangan |
|---|---|---|
| `foodSalesRevenue` | number (float) | Total pendapatan makanan dalam periode |
| `foodSalesCount` | number (int) | Jumlah order makanan dalam periode |

### 3.2 Objek baru `data.foodSales` (level top, sejajar dengan `revenue`, `sessions`, dll)

| Field | Tipe | Keterangan |
|---|---|---|
| `totalRevenue` | number (float) | Total pendapatan makanan dalam periode (sama dengan `revenue.foodSalesRevenue`) |
| `totalOrders` | number (int) | Total order makanan dalam periode |
| `averageOrderValue` | number (float) | Rata-rata nilai per order (`totalRevenue / totalOrders`) |
| `topItems` | array of `ReportFoodItem` | Daftar item menu terlaris dalam periode, diurutkan dari revenue terbesar |

**`ReportFoodItem`**:

| Field | Tipe | Keterangan |
|---|---|---|
| `itemName` | string | Nama item menu (mis. "Indomie Goreng") |
| `category` | string | Kategori menu (mis. "makanan", "minuman", "snack") |
| `quantitySold` | number (int) | Total qty terjual dalam periode |
| `revenue` | number (float) | Total pendapatan dari item ini dalam periode |

### 3.3 Field baru di setiap item `data.dailyBreakdown[]`

| Field | Tipe | Keterangan |
|---|---|---|
| `foodRevenue` | number (float) | Pendapatan makanan pada tanggal tersebut |
| `foodOrders` | number (int) | Jumlah order makanan pada tanggal tersebut |

### Contoh response lengkap (dipangkas untuk field yang tidak berubah)

```json
{
  "success": true,
  "message": "Laporan berhasil diambil",
  "data": {
    "period": { "startDate": "2026-07-17", "endDate": "2026-07-24", "totalDays": 8 },
    "revenue": {
      "totalRevenue": 3160666,
      "totalBaseAmount": 3300000,
      "voucherDiscount": 90000,
      "autoDiscount": 49334,
      "totalDiscount": 139334,
      "totalCashReceived": 3160666,
      "totalChange": 12000,
      "dailyRentalRevenue": 1200000,
      "dailyRentalCount": 6,
      "membershipRevenue": 800000,
      "membershipCount": 2,
      "foodSalesRevenue": 420000,
      "foodSalesCount": 34
    },
    "transactions": { "totalTransactions": 52, "voucherTransactions": 5, "averagePerDay": 6.5 },
    "sessions": { "totalSessions": 40, "totalPlayMinutes": 4800, "totalPlayHours": 80, "averageMinutes": 120 },
    "vouchers": [ /* ... */ ],
    "consoles": [ /* ... */ ],
    "dailyBreakdown": [
      {
        "date": "2026-07-24",
        "revenue": 450000,
        "transactions": 7,
        "sessions": 5,
        "playMinutes": 600,
        "foodRevenue": 60000,
        "foodOrders": 5
      }
    ],
    "activeDiscountRules": [ /* ... */ ],
    "foodSales": {
      "totalRevenue": 420000,
      "totalOrders": 34,
      "averageOrderValue": 12352.94,
      "topItems": [
        { "itemName": "Indomie Goreng", "category": "makanan", "quantitySold": 20, "revenue": 100000 },
        { "itemName": "Es Teh Manis", "category": "minuman", "quantitySold": 25, "revenue": 62500 }
      ]
    },
    "generatedAt": "2026-07-24T16:00:00Z"
  }
}
```

### Rekomendasi UI — Laporan

Buat **section terpisah** "Pendapatan Makanan/Minuman/Snack" (jangan dicampur ke section
pendapatan sewa/membership), berisi:

1. Ringkasan: `foodSales.totalRevenue`, `foodSales.totalOrders`, `foodSales.averageOrderValue`.
2. List "Item Terlaris": iterasi `foodSales.topItems`, tampilkan `itemName`, `category` (badge),
   `quantitySold`, `revenue`.
3. (Opsional) Grafik/tabel breakdown harian menggunakan `dailyBreakdown[].foodRevenue` dan
   `dailyBreakdown[].foodOrders` bila ingin tren pendapatan makanan per hari.
4. Pada ringkasan pendapatan utama, tetap tampilkan `revenue.foodSalesRevenue` sebagai salah satu
   baris breakdown (sejajar dengan "Sewa Harian", "Membership", "Voucher", dll) agar user tahu
   proporsi kontribusi makanan terhadap `revenue.totalRevenue`.

---

## 4. Ringkasan Perubahan Kontrak API (Checklist Frontend)

- [ ] `DashboardSummary`: tambahkan handling `foodSalesRevenue`, `foodOrderCount`, `pendingFoodOrders`.
- [ ] `ReportSummary.revenue`: tambahkan handling `foodSalesRevenue`, `foodSalesCount`.
- [ ] `ReportSummary`: tambahkan handling objek baru `foodSales` (`totalRevenue`, `totalOrders`,
      `averageOrderValue`, `topItems[]`).
- [ ] `ReportSummary.dailyBreakdown[]`: tambahkan handling `foodRevenue`, `foodOrders`.
- [ ] Dashboard: kartu baru "Pendapatan Makanan" (+ opsional kartu "Order Makanan Pending").
- [ ] Laporan: section baru "Pendapatan Makanan/Minuman/Snack" dengan ringkasan + item terlaris.
- [ ] Format mata uang konsisten dengan halaman lain (`Rp #.###` tanpa desimal).
- [ ] Tidak ada endpoint baru yang perlu dipanggil — cukup baca field tambahan dari 2 endpoint yang
      sudah dipakai sebelumnya.

---

## 5. Catatan Implementasi Backend (untuk referensi/debugging)

- Entity Go: [internal/domain/entity/dashboard.go](../internal/domain/entity/dashboard.go),
  [internal/domain/entity/report.go](../internal/domain/entity/report.go)
- Handler: [internal/delivery/http/handler/dashboard_handler.go](../internal/delivery/http/handler/dashboard_handler.go),
  [internal/delivery/http/handler/report_handler.go](../internal/delivery/http/handler/report_handler.go)
- Repository (query ke stored procedure): [internal/repository/postgres/payment_repository.go](../internal/repository/postgres/payment_repository.go)
- Stored procedure: `byoneDashboardSummary`, `byoneReportSummary` di [migrations/000002_procedures.up.sql](../migrations/000002_procedures.up.sql)
  (dimodifikasi via migration [migrations/000053_add_food_sales_to_reports.up.sql](../migrations/000053_add_food_sales_to_reports.up.sql))
- Sudah diverifikasi: `go build ./...` sukses, server berjalan dan endpoint mengembalikan data
  makanan yang valid saat diuji langsung via live server.
