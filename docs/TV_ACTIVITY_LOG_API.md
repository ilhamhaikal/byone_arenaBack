# 📺 Log Aktivitas TV — Dokumentasi Lengkap

> **Versi**: 2026-07-24 | **44 migrations** | Backend: Go + PostgreSQL

---

## 📊 Konsep Dasar

Sistem mencatat **semua aktivitas TV** dalam 2 cara:

| Sumber | Trigger | Event |
|---|---|---|
| **Sesi Rental** | Admin mulai/akhiri sesi | `ON`/`OFF` otomatis + durasi |
| **Heartbeat Client** | TV kirim status layar | `ON`/`OFF`/`sleep`/`screensaver` |

**Log SELALU tercatat**, baik TV dinyalakan manual (switch) maupun otomatis karena sesi.

---

## 📡 1. Heartbeat — Update Status TV (PUBLIK)

```
POST /api/v1/consoles/{consoleId}/heartbeat
Content-Type: application/json
{"screenStatus": "on"}
```

| `screenStatus` | Log tercatat |
|---|---|
| `"on"` | ✅ TV menyala |
| `"off"` | ✅ TV mati + durasi dari ON terakhir |
| `"sleep"` | ✅ TV sleep + durasi |
| `"screensaver"` | ✅ Screensaver + durasi |
| *(tidak dikirim)* | ❌ Hanya update `last_seen_at` |

### Response (200)
```json
{
  "success": true,
  "message": "TV menyala — SESI AKTIF",
  "data": {
    "logId": "74729ce4-...",
    "isAuthorized": true,
    "sessionId": "8774bfad-...",
    "durationMin": null
  }
}
```

> ⚠️ **PENTING UNTUK CLIENT**: Kirim `screenStatus` **hanya saat berubah**.  
> Kalau TV sudah ON, jangan kirim ON lagi. Kirim OFF hanya saat benar-benar mati.

---

## 📋 2. GetTvLogs — Ambil Log & Ringkasan

```
GET /api/v1/consoles/{consoleId}/tv-logs?date=YYYY-MM-DD
Authorization: Bearer {token}
```

### Response (200)

```json
{
  "success": true,
  "data": {
    "logs": [...],
    "unauthorizedCount": 2,
    "totalOnMinutes": 45,
    "authorizedMinutes": 30,
    "unauthorizedMinutes": 15,
    "activeSession": {
      "sessionId": "uuid",
      "startTime": "2026-07-24T10:30:00Z",
      "bookedMinutes": 60,
      "runningMinutes": 12.5,
      "status": "active",
      "customerName": "Budi"
    },
    "unauthorizedLogs": [...]
  }
}
```

### Penjelasan Field Ringkasan

| Field | Arti |
|---|---|
| `totalOnMinutes` | Total menit TV menyala (authorized + unauthorized) |
| `authorizedMinutes` | Menit live **dengan sesi** (termasuk sesi yang sedang berjalan) |
| `unauthorizedMinutes` | Menit live **tanpa sesi** (TV dinyalakan tanpa rental) |
| `unauthorizedCount` | Jumlah event ON tanpa sesi |
| `activeSession` | Info sesi aktif saat ini (`null` jika tidak ada) |
| `unauthorizedLogs` | List khusus entry unauthorized (max 20) |

### ⚠️ `logs` adalah Array, BUKAN String!

```javascript
// ✅ BENAR
data.logs.map(log => ...)

// ❌ SALAH  
JSON.parse(data.logs)  // akan error!
```

---

## 🔧 3. Aturan Authorization

| Situasi | `isAuthorized` |
|---|---|
| TV ON + ada sesi aktif | `true` ✅ |
| TV ON + tidak ada sesi | `false` ⚠️ unauthorized |
| TV OFF / sleep / screensaver | **inherit dari ON terakhir** |

> **OFF tidak mungkin unauthorized.** Kalau TV nyala tanpa sesi lalu dimatikan → OFF tetap `false` (mengikuti ON-nya), tapi durasi masuk ke `unauthorizedMinutes`.

---

## 💻 4. Contoh Fetch Frontend

```javascript
async function fetchTvLogs(consoleId, date) {
  const token = getAuthToken();
  const url = date
    ? `/api/v1/consoles/${consoleId}/tv-logs?date=${date}`
    : `/api/v1/consoles/${consoleId}/tv-logs`;

  const res = await fetch(url, {
    headers: { 'Authorization': `Bearer ${token}` }
  });
  const { success, data } = await res.json();

  if (!success) return { logs: [], unauthorizedLogs: [], unauthorizedCount: 0,
    totalOnMinutes: 0, authorizedMinutes: 0, unauthorizedMinutes: 0, activeSession: null };

  return {
    logs: data.logs || [],
    unauthorizedLogs: data.unauthorizedLogs || [],
    unauthorizedCount: data.unauthorizedCount || 0,
    totalOnMinutes: data.totalOnMinutes || 0,
    authorizedMinutes: data.authorizedMinutes || 0,
    unauthorizedMinutes: data.unauthorizedMinutes || 0,
    activeSession: data.activeSession || null,
  };
}
```

### Render UI

```jsx
function TvLogView({ data }) {
  return (
    <div>
      {/* Ringkasan */}
      <div className="summary">
        <span>🟢 Live: {data.authorizedMinutes} mnt</span>
        <span>⚠️ Unauth: {data.unauthorizedMinutes} mnt</span>
        <span>📊 Total: {data.totalOnMinutes} mnt</span>
      </div>

      {/* Sesi Aktif */}
      {data.activeSession && (
        <div className="active-session">
          🟢 SESI AKTIF — {Math.floor(data.activeSession.runningMinutes)} mnt berjalan
        </div>
      )}

      {/* Unauthorized */}
      {data.unauthorizedLogs.length > 0 && (
        <div className="unauthorized-list">
          <h3>⚠️ Unauthorized ({data.unauthorizedCount})</h3>
          {data.unauthorizedLogs.map(log => (
            <div key={log.id} className="unauthorized">
              {log.event.toUpperCase()} — {new Date(log.createdAt).toLocaleTimeString()}
            </div>
          ))}
        </div>
      )}

      {/* Semua Log */}
      {data.logs.map(log => (
        <div key={log.id} className={log.isAuthorized ? 'authorized' : 'unauthorized'}>
          {log.event === 'on' ? '🟢 ON' : '🔴 OFF'}
          {' '}{log.consoleName}
          {' '}{new Date(log.createdAt).toLocaleTimeString()}
          {log.durationMinutes && ` — ${log.durationMinutes} mnt`}
          {!log.isAuthorized && ' ⚠️'}
        </div>
      ))}
    </div>
  );
}
```

---

## 🔧 5. Troubleshooting

| Gejala | Penyebab | Solusi |
|---|---|---|
| Log kosong semua | Belum ada aktivitas TV | Mulai sesi atau kirim heartbeat ON |
| `authorizedMinutes` = 0 | Server belum restart | `go run cmd/server/main.go` |
| `unauthorizedMinutes` besar | TV dinyalakan tanpa sesi | Cek `unauthorizedLogs` |
| Log isinya OFF semua | Client kirim `off` terus | Client harus kirim `on` saat TV menyala |
| OFF masuk unauthorized | Bug lama — sudah difix | Update migration terbaru |
| 401 Unauthorized | Token JWT expired | Login ulang |

---

## 🗄️ 6. Database

### Trigger Auto-Log

| Trigger | Event | Keterangan |
|---|---|---|
| **Mulai sesi** | `ON` | Auto-insert saat sesi dimulai |
| **Akhiri sesi** | `OFF` + durasi | Auto-insert saat sesi selesai |
| **Heartbeat client** | `ON`/`OFF` | Manual dari TV |

---

## 📞 Test dengan curl

```bash
# Login
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['token'])")

# Heartbeat ON
curl -X POST http://localhost:8080/api/v1/consoles/{id}/heartbeat \
  -H "Content-Type: application/json" -d '{"screenStatus":"on"}'

# Heartbeat OFF
curl -X POST http://localhost:8080/api/v1/consoles/{id}/heartbeat \
  -H "Content-Type: application/json" -d '{"screenStatus":"off"}'

# Lihat log
curl -s "http://localhost:8080/api/v1/consoles/{id}/tv-logs?date=2026-07-24" \
  -H "Authorization: Bearer $TOKEN" | python3 -m json.tool | head -40
```
