# Backend Enhancement Requirements — Byone Arena

> **Tanggal**: 2026-07-20 (updated)  
> **Status**: ✅ Semua item selesai

---

## 1. Bug Fix — ✅

### Rental Harian FK Constraint
✅ `payments.session_id` nullable. SP `byoneCreateDailyRental` tidak insert payment dummy.

---

## 2. Console Fields — ✅

### dailyPrice
✅ Field di entity + create/update request.

### lastSeenAt + Heartbeat
✅ `POST /api/v1/consoles/{id}/heartbeat` (publik) → update `last_seen_at`.

---

## 3. Booking — ✅

| Endpoint | Response |
|---|---|
| `POST /bookings` | Full object + Console + Customer |
| `PATCH /bookings/{id}/status` | Full object + validasi (confirmed/cancelled/completed) |
| Validasi overlap | SP `byoneCreateBooking` |

---

## 4. Rental Harian — ✅

| Endpoint | Response |
|---|---|
| `POST /daily-rentals` | Full object + Console + Customer |
| `POST /daily-rentals/{id}/return` | Full object |
| Overdue auto-set | Goroutine 30 detik |

---

## 5. Membership — ✅

```
POST /api/v1/customers/{id}/membership
```

```json
{
  "membershipType": "lifetime",
  "membershipPrice": 50000,
  "cashReceived": 50000
}
```
- Lifetime (no expiry), monthly, yearly
- Price 0 = gratis
- Payment record hanya jika price > 0

---

## 6. Tiered Pricing — ✅

Console field `pricingTiers` (JSONB):
```json
[
  {"startMinute":0, "endMinute":60, "price":9000},
  {"startMinute":60, "endMinute":null, "price":8000}
]
```
SP `byoneCalculatePrice` + `byonePreviewPrice` response termasuk `priceBreakdown`.

---

## 7. All Endpoints

### Public
| Endpoint | Method |
|---|---|
| `/health` | GET |
| `/auth/login`, `/auth/register` | POST |
| `/consoles/overview` | GET |
| `/consoles/{id}/heartbeat` | POST |
| `/notifications` | GET |
| `/notifications/loop/status` | GET |
| `/ws` | WebSocket |

### Protected (JWT)
| Group | Endpoints |
|---|---|
| **Konsol** | CRUD + `/available` + `/{id}/price` + `/{id}/wake` + `/{id}/sleep` |
| **Sesi** | CRUD + `/start` + `/{id}/end` + `/{id}/cancel` + `/{id}/extend` |
| **Booking** | CRUD + `/{id}/status` |
| **Rental Harian** | CRUD + `/{id}/return` |
| **Pelanggan** | CRUD + `/{id}/membership` |
| **Pembayaran** | POST + `/{id}` + `/{id}/confirm` + `/{id}/refund` |
| **Dashboard** | `/summary` |
| **Laporan** | `/summary?startDate=&endDate=` |
| **Notifikasi** | POST/PUT/DELETE (admin) + `/loop/start` + `/loop/stop` |
| **Voucher** | CRUD + `/code/{code}` + `/{id}/toggle` |
| **Diskon** | CRUD + `/active` + `/{id}/toggle` |
| **Shift** | CRUD |
| **Menu** | CRUD + `/available` + `/{id}/toggle` |
| **Food Order** | CRUD + `/{id}/cancel` + `/{id}/status` |

---

## 8. Safety Nets

| Mechanism | Interval |
|---|---|
| Auto-stop expired sessions | 30 detik |
| Console stuck cleanup | 30 detik |
| Overdue daily rentals | 30 detik |
| Notification loop | 1 detik ticker |
| Duration cap on end session | Every call |
| All SP column-qualified | Compile-time |
