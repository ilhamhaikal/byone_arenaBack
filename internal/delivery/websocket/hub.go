package websocket

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"byone-arena/internal/domain/entity"
	"byone-arena/internal/recovery"
	"byone-arena/pkg/spname"

	"github.com/gofiber/contrib/websocket"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// EventType mendefinisikan tipe event yang disiarkan via WebSocket
type EventType string

const (
	EventSessionStarted   EventType = "SESSION_STARTED"
	EventSessionEnded     EventType = "SESSION_ENDED"
	EventSessionCancelled EventType = "SESSION_CANCELLED"
	EventSessionExtended  EventType = "SESSION_EXTENDED"
	EventPaymentCreated   EventType = "PAYMENT_CREATED"
	EventPaymentConfirmed EventType = "PAYMENT_CONFIRMED"
	EventPaymentRefunded  EventType = "PAYMENT_REFUNDED"
	EventConsoleUpdated   EventType = "CONSOLE_UPDATED"
	EventPing             EventType = "PING"

	EventTVWake         EventType = "TV_WAKE"
	EventTVSleep        EventType = "TV_SLEEP"
	EventTVScreensaver  EventType = "TV_SCREENSAVER"
	EventTVNotification EventType = "TV_NOTIFICATION"
)

type Event struct {
	Type      EventType   `json:"type"`
	Payload   interface{} `json:"payload"`
	Timestamp time.Time   `json:"timestamp"`
	// ConsoleID menandai event hanya untuk satu konsol. nil = global.
	ConsoleID *uuid.UUID `json:"consoleId,omitempty"`
	// TargetConsoleIDs dipakai untuk event yang menargetkan beberapa konsol.
	// Tidak dikirim ke client (hanya untuk routing di server).
	TargetConsoleIDs []uuid.UUID `json:"-"`
}

func NewEvent(eventType EventType, payload interface{}) *Event {
	return &Event{Type: eventType, Payload: payload, Timestamp: time.Now()}
}

type client struct {
	conn      *websocket.Conn
	send      chan []byte
	logger    *zap.Logger
	consoleID uuid.UUID
}

type broadcastMessage struct {
	event *Event
	data  []byte
}

type Hub struct {
	mu            sync.RWMutex
	clients       map[*client]bool
	broadcast     chan broadcastMessage
	register      chan *client
	unregister    chan *client
	logger        *zap.Logger
	notifyStop    chan struct{}
	notifyRunning bool
	db            *gorm.DB
}

func NewHub(logger *zap.Logger) *Hub {
	return &Hub{
		clients:    make(map[*client]bool),
		broadcast:  make(chan broadcastMessage, 256),
		register:   make(chan *client),
		unregister: make(chan *client),
		logger:     logger,
		notifyStop: make(chan struct{}),
	}
}

func (h *Hub) SetDB(db *gorm.DB) { h.db = db }

func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			h.clients[c] = true
			h.mu.Unlock()
		case c := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.RLock()
			for c := range h.clients {
				if !canReceive(c, message.event) {
					continue
				}
				select {
				case c.send <- message.data:
				default:
					close(c.send)
					delete(h.clients, c)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// canReceive membatasi event yang boleh diterima satu client TV.
// Saat ini semua koneksi WS adalah TV, jadi event admin (payment/console)
// tidak dikirim. Event per konsol hanya sampai ke konsol yang sesuai.
func canReceive(c *client, event *Event) bool {
	if !isTVEvent(event.Type) {
		return false
	}
	if event.ConsoleID != nil && c.consoleID != *event.ConsoleID {
		return false
	}
	if len(event.TargetConsoleIDs) > 0 {
		for _, id := range event.TargetConsoleIDs {
			if id == c.consoleID {
				return true
			}
		}
		return false
	}
	return true
}

func isTVEvent(t EventType) bool {
	switch t {
	case EventSessionStarted, EventSessionEnded, EventSessionCancelled,
		EventSessionExtended, EventTVWake, EventTVSleep, EventTVScreensaver,
		EventTVNotification:
		return true
	default:
		return string(t) == "food_order_preparing"
	}
}

func (h *Hub) Broadcast(event *Event) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	h.broadcast <- broadcastMessage{event: event, data: data}
}

func (h *Hub) HandleConnection(conn *websocket.Conn, consoleID uuid.UUID) {
	c := &client{
		conn:      conn,
		send:      make(chan []byte, 256),
		logger:    h.logger,
		consoleID: consoleID,
	}
	h.register <- c

	// conn.Close() bisa dipicu dari 2 arah (writer goroutine saat write gagal,
	// atau read loop di bawah saat selesai) — sync.Once mencegah double-close.
	var closeOnce sync.Once
	closeConn := func() { closeOnce.Do(func() { conn.Close() }) }

	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		for message := range c.send {
			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				// Write gagal (koneksi putus) → paksa read loop di bawah ikut berhenti.
				closeConn()
				return
			}
		}
	}()

	for {
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if msgType == websocket.TextMessage {
			var incoming Event
			if err := json.Unmarshal(msg, &incoming); err == nil && incoming.Type == EventPing {
				pong := NewEvent(EventPing, map[string]string{"status": "pong"})
				if data, err := json.Marshal(pong); err == nil {
					select {
					case c.send <- data:
					default:
						// buffer penuh — client kemungkinan sudah lambat/putus, jangan blok read loop
					}
				}
			}
		}
	}

	// Read loop selesai (client disconnect/error). Unregister supaya hub
	// berhenti mem-broadcast ke client ini (ini yang menutup channel c.send),
	// lalu TUNGGU writer goroutine benar-benar selesai sebelum function ini
	// return.
	//
	// PENTING untuk fasthttp/websocket: begitu handler function ini return,
	// fasthttp bisa langsung reuse/reset koneksi di bawahnya. Sebelumnya,
	// writer goroutine dibiarkan "orphan" — baru sadar koneksi putus saat
	// write berikutnya gagal, yang bisa terjadi lama setelah HandleConnection
	// sudah return. Itu penyebab panic "concurrent write ke koneksi yang
	// sudah di-reuse/invalid". Sekarang kita pastikan writer goroutine
	// selesai dulu sebelum function ini return.
	h.unregister <- c
	closeConn()
	<-writerDone
}

func (h *Hub) StartNotificationLoop() {
	if h.db == nil {
		return
	}
	h.mu.Lock()
	if h.notifyRunning {
		h.mu.Unlock()
		return
	}
	h.notifyRunning = true
	h.notifyStop = make(chan struct{})
	h.mu.Unlock()

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		lastSent := make(map[uuid.UUID]time.Time)

		for {
			select {
			case <-h.notifyStop:
				return
			case tick := <-ticker.C:
				if h.db == nil {
					continue
				}
				var notifications []entity.TvNotification
				h.db.Where("loop_enabled = ? AND is_active = ? AND auto_show = ?", true, true, true).Find(&notifications)
				for _, n := range notifications {
					last, exists := lastSent[n.ID]
					interval := time.Duration(n.LoopInterval) * time.Second
					if interval < 5*time.Second {
						interval = 5 * time.Second
					}
					if !exists || tick.Sub(last) >= interval {
						evt := NewEvent(EventTVNotification, map[string]interface{}{
							"id": n.ID, "title": n.Title, "message": n.Message,
							"imageUrl": n.ImageURL, "priority": n.Priority,
							"targetAll": n.TargetAll, "targetConsoleIds": n.TargetConsoleIDs,
							"activeSessionsOnly": n.ActiveSessionsOnly,
							"autoShow":           n.AutoShow,
							"loopEnabled":        n.LoopEnabled,
							"loopInterval":       n.LoopInterval,
							"lastTriggeredAt":    n.LastTriggeredAt,
							"targetConsoleType":  n.TargetConsoleType,
						})
						if !n.TargetAll {
							for _, raw := range n.TargetConsoleIDs {
								if id, err := uuid.Parse(raw); err == nil {
									evt.TargetConsoleIDs = append(evt.TargetConsoleIDs, id)
								}
							}
							if len(evt.TargetConsoleIDs) == 0 {
								lastSent[n.ID] = tick
								continue
							}
						}
						h.Broadcast(evt)
						lastSent[n.ID] = tick
					}
				}
			}
		}
	}()
}

func (h *Hub) StopNotificationLoop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.notifyRunning {
		close(h.notifyStop)
		h.notifyRunning = false
	}
}

func (h *Hub) IsNotificationRunning() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.notifyRunning
}

func (h *Hub) BroadcastToConsole(consoleID uuid.UUID, event *Event) {
	event.ConsoleID = &consoleID
	h.Broadcast(event)
}

func (h *Hub) StartAutoStop() {
	if h.db == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if h.db == nil {
				continue
			}
			// Jalankan maintenance via SP (cleanup status + overdue)
			h.db.Exec(fmt.Sprintf("SELECT %s()", spname.Ident("RunMaintenance")))

			// Cari sesi expired → auto-stop
			var expiredSessions []struct {
				ID        uuid.UUID `gorm:"column:id"`
				ConsoleID uuid.UUID `gorm:"column:console_id"`
			}
			if err := h.db.Raw(
				fmt.Sprintf("SELECT * FROM %s()", spname.Ident("GetExpiredSessions")),
			).Scan(&expiredSessions).Error; err != nil {
				h.logger.Warn("auto-stop: gagal membaca sesi kedaluwarsa", zap.Error(err))
				continue
			}
			for _, s := range expiredSessions {
				// Listrik mati / TV offline sebelum jadwal selesai → interrupted,
				// sisa waktu bisa dipulihkan admin lewat voucher GRATIS.
				if recovery.ShouldInterrupt(h.db, s.ID) {
					if _, err := recovery.Interrupt(h.db, s.ID, "power_outage"); err != nil {
						h.logger.Warn("auto-stop: gagal menandai sesi interrupted",
							zap.String("session", s.ID.String()), zap.Error(err))
						continue
					}
				} else {
					if err := h.db.Exec(
						fmt.Sprintf(`SELECT %s(?)`, spname.Ident("EndSession")), s.ID,
					).Error; err != nil {
						h.logger.Warn("auto-stop: gagal mengakhiri sesi",
							zap.String("session", s.ID.String()), zap.Error(err))
						continue
					}
				}

				// Hanya ubah konsol & broadcast jika penutupan sesi berhasil.
				if err := h.db.Model(&entity.Console{}).
					Where("id = ?", s.ConsoleID).
					Updates(map[string]interface{}{
						"status":        entity.ConsoleStatusAvailable,
						"screen_status": entity.ScreenStatusOff,
						"manual_live":   false,
					}).Error; err != nil {
					h.logger.Warn("auto-stop: sesi ditutup tetapi reset konsol gagal",
						zap.String("session", s.ID.String()), zap.Error(err))
				}
				h.BroadcastSessionEnded(map[string]interface{}{
					"sessionId": s.ID, "consoleId": s.ConsoleID, "reason": "auto_stop",
				}, s.ConsoleID)
				// Waktu habis → TV client tampilkan screensaver Byone Arena kembali
				h.BroadcastTVScreensaver(s.ConsoleID)
			}
		}
	}()
}
