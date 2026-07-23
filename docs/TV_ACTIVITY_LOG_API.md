# Dokumentasi Endpoint: Log Aktivitas TV

> **Versi**: 2026-07-23 | **34 migrations applied** | Backend: Go + PostgreSQL

---

## ⚠️ Ringkasan Perubahan Terbaru (Wajib dibaca Frontend)

| # | Perubahan | Impact Frontend |
|---|---|---|
| 1 | `logs` sekarang **array JSON asli**, bukan string | Tidak perlu `JSON.parse()` lagi |
| 2 | Response ada **`activeSession`** — info sesi aktif + running duration | Tampilkan header "Sesi aktif: X menit" |
| 3 | Response ada **`unauthorizedLogs`** — list khusus unauthorized | Tampilkan section "⚠️ Unauthorized" terpisah |
| 4 | Setiap log entry punya **`isAuthorized`** (bool) | Render badge hijau/merah |

---

## 1. Heartbeat — Kirim Status TV (PUBLIK, no auth)

## 1. Heartbeat — Kirim Status TV

TV/Android app mengirim heartbeat setiap ~10 detik untuk update status.

### Request

```
POST /api/v1/consoles/{consoleId}/heartbeat
Content-Type: application/json
```

**Body:**
```json
{
  "screenStatus": "on"
}
```

| `screenStatus` | Keterangan |
|---|---|
| `"on"` | TV menyala → dicatat sebagai log |
| `"off"` | TV mati → dicatat + hitung durasi |
| `"sleep"` | TV sleep → dicatat + hitung durasi |
| `"screensaver"` | Screensaver → dicatat + hitung durasi |
| *(tidak dikirim)* | Hanya update `last_seen_at`, tidak insert log |

### Response (200 OK)

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

| Field | Type | Keterangan |
|---|---|---|
| `logId` | UUID | ID log yang baru dibuat |
| `isAuthorized` | bool | `true` = ada sesi aktif, `false` = unauthorized |
| `sessionId` | UUID/null | ID sesi aktif (null jika unauthorized) |
| `durationMin` | int/null | Durasi TV menyala (hanya untuk event off/sleep/screensaver) |

### Contoh Tanpa Sesi (Unauthorized)
```json
{
  "success": true,
  "message": "⚠️ TV menyala — TANPA SESI (unauthorized)",
  "data": {
    "logId": "01974d8b-...",
    "isAuthorized": false,
    "sessionId": null,
    "durationMin": null
  }
}
```

---

## 2. GetTvLogs — Ambil Log Aktivitas TV

### Request

```
GET /api/v1/consoles/{consoleId}/tv-logs?date=YYYY-MM-DD
Authorization: Bearer {token}
```

| Param | Wajib | Keterangan |
|---|---|---|
| `consoleId` | ✅ | UUID konsol |
| `date` | ❌ | Filter tanggal (default: semua) |

### Response (200 OK)

```json
{
  "success": true,
  "message": "Log aktivitas TV",
  "data": {
    "logs": [
      {
        "id": "e2d88cf6-...",
        "event": "on",
        "isAuthorized": true,
        "durationMinutes": null,
        "sessionId": "8774bfad-...",
        "createdAt": "2026-07-23T13:43:07Z",
        "consoleName": "PS4 SLIM 1TB",
        "sort_order": "2026-07-23T13:43:07.49137+07:00"
      }
    ],
    "unauthorizedCount": 6,
    "totalOnMinutes": 0,
    "activeSession": {
      "sessionId": "8774bfad-...",
      "startTime": "2026-07-23T12:30:00Z",
      "bookedMinutes": 60,
      "runningMinutes": 72.5,
      "status": "active",
      "customerName": "Budi"
    }
  }
}
```

| Field | Type | Keterangan |
|---|---|---|
| `logs` | **Array** | ⚠️ Array JSON asli (bukan string!), langsung bisa di-loop |
| `logs[].id` | UUID | ID log |
| `logs[].event` | string | `"on"`, `"off"`, `"sleep"`, `"screensaver"` |
| `logs[].isAuthorized` | bool | TV nyala dengan sesi? |
| `logs[].durationMinutes` | int/null | Durasi (hanya event off/sleep/screensaver) |
| `logs[].sessionId` | UUID/null | ID sesi terkait |
| `logs[].createdAt` | string | Timestamp UTC format ISO 8601 |
| `logs[].consoleName` | string | Nama konsol |
| `unauthorizedCount` | int | Total TV nyala tanpa sesi |
| `totalOnMinutes` | int | Total menit TV menyala |
| `activeSession` | object/null | **Info sesi aktif saat ini** (null jika tidak ada) |
| `activeSession.sessionId` | UUID | ID sesi |
| `activeSession.startTime` | string | Waktu mulai sesi |
| `activeSession.bookedMinutes` | int | Durasi booking awal |
| `activeSession.runningMinutes` | float | **Durasi berjalan real-time (menit)** |
| `activeSession.customerName` | string | Nama pelanggan |

---

## 3. Implementasi Frontend

### ⚠️ PENTING: `logs` adalah Array, bukan String!

**Response backend sudah diperbaiki** (setelah migration 000031 + handler fix):
- `data.logs` = **`[...]`** (array) ✅
- BUKAN `"[...]"` (string) ❌

### Contoh Fetch (JavaScript/React)

```javascript
async function fetchTvLogs(consoleId, date) {
  const token = getAuthToken(); // dari login
  const url = date 
    ? `/api/v1/consoles/${consoleId}/tv-logs?date=${date}`
    : `/api/v1/consoles/${consoleId}/tv-logs`;

  const res = await fetch(url, {
    headers: { 'Authorization': `Bearer ${token}` }
  });
  const json = await res.json();

  if (!json.success) {
    console.error('Gagal:', json.message);
    return { logs: [], unauthorizedCount: 0, totalOnMinutes: 0 };
  }

  // ⚠️ Pastikan logs adalah array
  const logs = Array.isArray(json.data.logs) 
    ? json.data.logs 
    : JSON.parse(json.data.logs); // fallback jika masih string

  return {
    logs,
    unauthorizedCount: json.data.unauthorizedCount,
    totalOnMinutes: json.data.totalOnMinutes,
  };
}
```

### Render di UI

```jsx
function TvLogView({ consoleId }) {
  const [logs, setLogs] = useState([]);
  const [date, setDate] = useState('2026-07-23');

  useEffect(() => {
    fetchTvLogs(consoleId, date).then(data => {
      if (data.logs.length === 0) {
        // Tampilkan "Tidak ada log untuk tanggal ini"
      }
      setLogs(data.logs);
    });
  }, [consoleId, date]);

  return (
    <div>
      {logs.length === 0 ? (
        <p>Tidak ada log untuk tanggal ini</p>
      ) : (
        logs.map(log => (
          <div key={log.id} className={log.isAuthorized ? 'authorized' : 'unauthorized'}>
            <span>{log.event === 'on' ? '🟢 NYALA' : '🔴 MATI'}</span>
            <span>{log.consoleName}</span>
            <span>{new Date(log.createdAt).toLocaleTimeString()}</span>
            {!log.isAuthorized && <span className="badge">⚠️ Unauthorized</span>}
            {log.durationMinutes && <span>{log.durationMinutes} menit</span>}
          </div>
        ))
      )}
    </div>
  );
}
```

### Kirim Heartbeat (dari Android TV / Client)

```javascript
// Di ClientProvider._poll() — setiap ~10 detik
async function sendHeartbeat(consoleId, screenStatus) {
  await fetch(`/api/v1/consoles/${consoleId}/heartbeat`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ screenStatus })  // "on" | "off" | "sleep" | "screensaver"
  });
}
```

---

## 4. Troubleshooting

| Gejala | Penyebab | Solusi |
|---|---|---|
| Log kosong ("Tidak ada log") | Frontend tidak parse `data.logs` sebagai array | Gunakan `Array.isArray()` check |
| Log kosong padahal heartbeat jalan | TV kirim heartbeat tanpa `screenStatus` | Pastikan body ada `{"screenStatus":"on"}` |
| `isAuthorized` selalu false | Tidak ada sesi aktif untuk konsol itu | Buat sesi via endpoint Start Session |
| `totalOnMinutes` = 0 | Belum ada event `off`/`sleep`/`screensaver` | Kirim heartbeat dengan `screenStatus:"off"` saat TV dimatikan |
| `unauthorizedCount` > 0 | TV dinyalakan tanpa sesi rental | Normal — catat sebagai peringatan |
| 401 Unauthorized | Token JWT tidak dikirim | Tambahkan header `Authorization: Bearer {token}` |
| 400 Bad Request | Console ID format salah | Gunakan UUID valid dari `/api/v1/consoles/overview` |

---

## 5. Database

### Tabel: `tv_activity_logs`

| Kolom | Type | Keterangan |
|---|---|---|
| `id` | UUID | PK |
| `console_id` | UUID | FK → consoles |
| `event` | VARCHAR(20) | `on`, `off`, `sleep`, `screensaver` |
| `session_id` | UUID/null | FK → sessions |
| `is_authorized` | BOOLEAN | TV menyala saat ada sesi aktif? |
| `duration_minutes` | INT/null | Durasi (hanya untuk event off) |
| `created_at` | TIMESTAMPTZ | Waktu kejadian |

### Stored Procedures

| SP | Keterangan |
|---|---|
| `byoneLogTvActivity(console_id, event)` | Insert log + cek otorisasi |
| `byoneGetTvLogs(console_id, date?)` | Query log + summary |

---

## 6. Test dengan curl

```bash
# 1. Login (dapat token)
curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'

# 2. Cari console ID
curl -s http://localhost:8080/api/v1/consoles/overview

# 3. Kirim heartbeat (TV nyala)
curl -X POST http://localhost:8080/api/v1/consoles/{consoleId}/heartbeat \
  -H "Content-Type: application/json" \
  -d '{"screenStatus":"on"}'

# 4. Ambil log (pakai token dari step 1)
curl -s "http://localhost:8080/api/v1/consoles/{consoleId}/tv-logs?date=2026-07-23" \
  -H "Authorization: Bearer {token}" | python3 -m json.tool
```
