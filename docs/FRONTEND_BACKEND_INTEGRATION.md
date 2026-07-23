# Dokumentasi Integrasi Frontend ↔ Backend — BYONE ARENA

> **Backend adalah source of truth. Frontend mengikuti backend.**

---

## 1. Konsol Overview (Dashboard + Rental)
```
GET /api/v1/consoles/overview
```
**Auth**: PUBLIC  
**screenStatus**: `"on"` | `"off"` (default: `"off"`)

---

## 2. TV ON (Nyalakan Layar)
```
POST /api/v1/consoles/{id}/wake
```
**Auth**: Bearer token | Body: _kosong_  
**Efek**: `screenStatus` → `"on"` + broadcast WebSocket `TV_WAKE`

---

## 3. TV OFF (Matikan Layar)
```
POST /api/v1/consoles/{id}/sleep
```
**Auth**: Bearer token | Body: _kosong_  
**Efek**: `screenStatus` → `"off"` + broadcast WebSocket `TV_SLEEP`

---

## 4. TV Heartbeat (Client TV → Server)
```
POST /api/v1/consoles/{id}/heartbeat
```
**Auth**: PUBLIC  
**Body**: `{"screenStatus": "on" | "off" | "sleep" | "screensaver"}`  
**Efek**: Update `lastSeenAt` + log aktivitas via SP `byoneLogTvActivity`

---

## 5. TV Logs (Admin lihat log aktivitas TV)
```
GET /api/v1/consoles/{id}/tv-logs?date=YYYY-MM-DD
```
**Auth**: Bearer token  
**Response**: `{ logs: [...], unauthorizedCount: N, totalOnMinutes: N }`  
- `action`: `"on"` | `"off"` | `"sleep"` | `"screensaver"`  
- `unauthorized`: `true` = TV nyala tanpa sesi (PELANGGARAN)

---

## 6. Mulai Sesi
```
POST /api/v1/sessions/start
```
**Auth**: Bearer token  
**Body**: `{ consoleId, customerId, bookedDurationMinutes, cashReceived, voucherCode, notes }`  
**Min durasi**: 1 menit  
**Efek**: Sesi aktif + `screenStatus` → `"on"` + broadcast `SESSION_STARTED`

---

## 7. Extend Session (Tambah Waktu)
```
POST /api/v1/sessions/{id}/extend
```
**Auth**: Bearer token  
**Body**: `{ additionalMinutes, payNow, cashReceived, voucherCode, notes }`  
- `payNow: true` → `cashReceived` wajib > 0, payment = PAID  
- `payNow: false` → `cashReceived` opsional, payment = PENDING  
**Min tambahan**: 1 menit  
**Efek**: broadcast `SESSION_EXTENDED` + insert notifikasi TV

---

## 8. Akhiri Sesi
```
PATCH /api/v1/sessions/{id}/end
```
**Auth**: Bearer token | Body: _kosong_  
**Otomatis**: Jika ada pending payment → insert notifikasi "Pembayaran Tertunda"

---

## 9. Pending Payments
```
GET /api/v1/payments/pending
```
**Auth**: Bearer token  
**Response**: `{ pendingCount: N, payments: [...] }`

---

## 10. Konfirmasi Pembayaran
```
POST /api/v1/payments/{id}/confirm
```
**Auth**: Bearer token | Body: _kosong_

---

## 11. Dashboard Summary
```
GET /api/v1/dashboard/summary?date=YYYY-MM-DD
```
**Auth**: Bearer token  
Fields: `totalRevenue`, `dailyRentalRevenue`, `membershipRevenue`, `voucherUsageCount`, dll.

---

## 12. Report Summary
```
GET /api/v1/reports/summary?startDate=YYYY-MM-DD&endDate=YYYY-MM-DD
```
**Auth**: Bearer token  
Breakdown: `dailyRentalRevenue`, `membershipRevenue`, `vouchers[]`, `dailyBreakdown[]`

---

## 13. Aktivitas Terbaru
```
GET /api/v1/activities/recent?limit=10
```
**Auth**: Bearer token  
**Response**: `[ { type, action, title, detail, timestamp } ]`  
**type**: `console` | `setting` | `session` | `daily_rental` | `membership` | `payment`

---

## 14. Rental Harian
```
POST   /api/v1/daily-rentals
GET    /api/v1/daily-rentals
POST   /api/v1/daily-rentals/{id}/return
```
Fields: `totalDays`, `freeDaysUsed`, `discountAmount`, `finalAmount`, `voucherId`

---

## 15. Voucher
```
CRUD   /api/v1/vouchers
```
**discountType**: `"percentage"` | `"fixed_amount"` | `"free_days"`

---

## 16. Membership
```
POST /api/v1/customers/{id}/membership
```
Body: `{"cashReceived": 50000}` (opsional)

---

## Auth
```
POST /api/v1/auth/login    → { token, user }
POST /api/v1/auth/register → { token, user }
```
Header: `Authorization: Bearer <token>`
