package websocket

import "github.com/google/uuid"

// broadcaster.go — Lapisan tunggal yang memisahkan bentuk (shape) payload
// event WebSocket dari logika bisnis HTTP handler. Handler hanya perlu
// memanggil method bernama jelas di sini (mis. BroadcastSessionStarted),
// tanpa perlu tahu detail konstruksi Event/payload — menjaga pemisahan
// tanggung jawab antara transport HTTP (REST) dan transport realtime (WS).

// BroadcastSessionStarted mengumumkan sesi rental baru dimulai + auto-wake TV.
func (h *Hub) BroadcastSessionStarted(payload interface{}, consoleID uuid.UUID) {
	h.Broadcast(NewEvent(EventSessionStarted, payload))
	h.Broadcast(NewEvent(EventTVWake, map[string]interface{}{"consoleId": consoleID}))
}

// BroadcastSessionEnded mengumumkan sesi rental berakhir + auto-sleep TV.
func (h *Hub) BroadcastSessionEnded(payload interface{}, consoleID uuid.UUID) {
	h.Broadcast(NewEvent(EventSessionEnded, payload))
	h.Broadcast(NewEvent(EventTVSleep, map[string]interface{}{"consoleId": consoleID}))
}

// BroadcastSessionCancelled mengumumkan sesi rental dibatalkan.
func (h *Hub) BroadcastSessionCancelled(sessionID uuid.UUID) {
	h.Broadcast(NewEvent(EventSessionCancelled, map[string]interface{}{"session_id": sessionID}))
}

// BroadcastSessionExtended mengumumkan penambahan waktu sewa pada sesi aktif.
func (h *Hub) BroadcastSessionExtended(payload interface{}) {
	h.Broadcast(NewEvent(EventSessionExtended, payload))
}

// BroadcastPaymentCreated mengumumkan pembayaran baru berhasil dibuat.
func (h *Hub) BroadcastPaymentCreated(payload interface{}) {
	h.Broadcast(NewEvent(EventPaymentCreated, payload))
}

// BroadcastPaymentConfirmed mengumumkan pembayaran pending telah dikonfirmasi lunas.
func (h *Hub) BroadcastPaymentConfirmed(payload interface{}) {
	h.Broadcast(NewEvent(EventPaymentConfirmed, payload))
}

// BroadcastPaymentRefunded mengumumkan pembayaran telah direfund.
func (h *Hub) BroadcastPaymentRefunded(payload interface{}) {
	h.Broadcast(NewEvent(EventPaymentRefunded, payload))
}

// BroadcastConsoleUpdated mengumumkan perubahan data konsol (harga, status, dsb).
func (h *Hub) BroadcastConsoleUpdated(payload interface{}) {
	h.Broadcast(NewEvent(EventConsoleUpdated, payload))
}

// BroadcastTVWake memerintahkan TV untuk menyala/tampil (keluar dari screensaver).
func (h *Hub) BroadcastTVWake(consoleID uuid.UUID, ipAddress *string) {
	h.Broadcast(NewEvent(EventTVWake, map[string]interface{}{
		"consoleId": consoleID, "ipAddress": ipAddress,
	}))
}

// BroadcastTVSleep memerintahkan TV untuk mati layar / kembali idle.
func (h *Hub) BroadcastTVSleep(consoleID uuid.UUID, ipAddress *string) {
	h.Broadcast(NewEvent(EventTVSleep, map[string]interface{}{
		"consoleId": consoleID, "ipAddress": ipAddress,
	}))
}

// BroadcastTVScreensaver memerintahkan TV untuk menampilkan mode screensaver
// (dipakai saat sesi berakhir otomatis/waktu habis, atau dipicu manual admin).
func (h *Hub) BroadcastTVScreensaver(consoleID uuid.UUID) {
	h.Broadcast(NewEvent(EventTVScreensaver, map[string]interface{}{"consoleId": consoleID}))
}

// BroadcastTVNotification mengirim notifikasi promosi/pesan ke TV client.
func (h *Hub) BroadcastTVNotification(payload interface{}) {
	h.Broadcast(NewEvent(EventTVNotification, payload))
}
