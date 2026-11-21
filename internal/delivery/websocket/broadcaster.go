package websocket

import (
	"byone-arena/internal/domain/entity"

	"github.com/google/uuid"
)

// broadcaster.go — Lapisan tunggal yang memisahkan bentuk (shape) payload
// event WebSocket dari logika bisnis HTTP handler. Handler hanya perlu
// memanggil method bernama jelas di sini (mis. BroadcastSessionStarted),
// tanpa perlu tahu detail konstruksi Event/payload — menjaga pemisahan
// tanggung jawab antara transport HTTP (REST) dan transport realtime (WS).

// eventForConsole membuat event yang hanya ditujukan ke satu konsol.
func eventForConsole(t EventType, payload interface{}, consoleID uuid.UUID) *Event {
	e := NewEvent(t, payload)
	e.ConsoleID = &consoleID
	return e
}

// BroadcastSessionStarted mengumumkan sesi rental baru dimulai + auto-wake TV.
func (h *Hub) BroadcastSessionStarted(payload interface{}, consoleID uuid.UUID) {
	h.Broadcast(eventForConsole(EventSessionStarted, payload, consoleID))
	h.Broadcast(eventForConsole(EventTVWake, map[string]interface{}{"consoleId": consoleID}, consoleID))
}

// BroadcastSessionEnded mengumumkan sesi rental berakhir + auto-sleep TV.
func (h *Hub) BroadcastSessionEnded(payload interface{}, consoleID uuid.UUID) {
	h.Broadcast(eventForConsole(EventSessionEnded, payload, consoleID))
	h.Broadcast(eventForConsole(EventTVSleep, map[string]interface{}{"consoleId": consoleID}, consoleID))
}

// BroadcastSessionCancelled mengumumkan sesi rental dibatalkan.
func (h *Hub) BroadcastSessionCancelled(sessionID, consoleID uuid.UUID) {
	h.Broadcast(eventForConsole(EventSessionCancelled, map[string]interface{}{
		"session_id": sessionID, "consoleId": consoleID,
	}, consoleID))
}

// BroadcastSessionExtended mengumumkan penambahan waktu sewa pada sesi aktif.
func (h *Hub) BroadcastSessionExtended(payload interface{}, consoleID uuid.UUID) {
	h.Broadcast(eventForConsole(EventSessionExtended, payload, consoleID))
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
	h.Broadcast(eventForConsole(EventTVWake, map[string]interface{}{
		"consoleId": consoleID, "ipAddress": ipAddress,
	}, consoleID))
}

// BroadcastTVSleep memerintahkan TV untuk mati layar / kembali idle.
func (h *Hub) BroadcastTVSleep(consoleID uuid.UUID, ipAddress *string) {
	h.Broadcast(eventForConsole(EventTVSleep, map[string]interface{}{
		"consoleId": consoleID, "ipAddress": ipAddress,
	}, consoleID))
}

// BroadcastTVScreensaver memerintahkan TV untuk menampilkan mode screensaver
// (dipakai saat sesi berakhir otomatis/waktu habis, atau dipicu manual admin).
func (h *Hub) BroadcastTVScreensaver(consoleID uuid.UUID) {
	h.Broadcast(eventForConsole(EventTVScreensaver, map[string]interface{}{
		"consoleId": consoleID,
	}, consoleID))
}

// BroadcastTVNotification mengirim notifikasi promosi/pesan ke TV client.
func (h *Hub) BroadcastTVNotification(payload interface{}) {
	event := NewEvent(EventTVNotification, payload)
	switch n := payload.(type) {
	case *entity.TvNotification:
		if n != nil && !n.TargetAll {
			for _, raw := range n.TargetConsoleIDs {
				if id, err := uuid.Parse(raw); err == nil {
					event.TargetConsoleIDs = append(event.TargetConsoleIDs, id)
				}
			}
		}
	case entity.TvNotification:
		if !n.TargetAll {
			for _, raw := range n.TargetConsoleIDs {
				if id, err := uuid.Parse(raw); err == nil {
					event.TargetConsoleIDs = append(event.TargetConsoleIDs, id)
				}
			}
		}
	}
	// Notifikasi bertarget tetapi tidak punya target valid tidak boleh
	// berubah menjadi broadcast global.
	if n, ok := payload.(*entity.TvNotification); ok && n != nil &&
		!n.TargetAll && len(event.TargetConsoleIDs) == 0 {
		return
	}
	h.Broadcast(event)
}
