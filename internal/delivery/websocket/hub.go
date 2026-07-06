package websocket

import (
	"encoding/json"
	"sync"
	"time"

	"byone-arena/internal/domain/entity"

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
	EventPaymentCreated   EventType = "PAYMENT_CREATED"
	EventPaymentConfirmed EventType = "PAYMENT_CONFIRMED"
	EventPaymentRefunded  EventType = "PAYMENT_REFUNDED"
	EventConsoleUpdated   EventType = "CONSOLE_UPDATED"
	EventPing             EventType = "PING"

	// TV Control events
	EventTVWake        EventType = "TV_WAKE"
	EventTVSleep       EventType = "TV_SLEEP"
	EventTVScreensaver EventType = "TV_SCREENSAVER"
	EventTVNotification EventType = "TV_NOTIFICATION"
)

// Event adalah struktur pesan yang dikirimkan melalui WebSocket
type Event struct {
	Type      EventType   `json:"type"`
	Payload   interface{} `json:"payload"`
	Timestamp time.Time   `json:"timestamp"`
}

// NewEvent membuat event baru dengan timestamp sekarang
func NewEvent(eventType EventType, payload interface{}) *Event {
	return &Event{
		Type:      eventType,
		Payload:   payload,
		Timestamp: time.Now(),
	}
}

// client merepresentasikan satu koneksi WebSocket yang aktif
type client struct {
	conn   *websocket.Conn
	send   chan []byte
	logger *zap.Logger
}

// Hub mengelola semua koneksi WebSocket aktif dan mendistribusikan pesan
type Hub struct {
	mu         sync.RWMutex
	clients    map[*client]bool
	broadcast  chan []byte
	register   chan *client
	unregister chan *client
	logger     *zap.Logger

	// Notification loop
	notifyStop    chan struct{}
	notifyRunning bool
	db            *gorm.DB
}

// NewHub membuat instance baru Hub
func NewHub(logger *zap.Logger) *Hub {
	return &Hub{
		clients:    make(map[*client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *client),
		unregister: make(chan *client),
		logger:     logger,
		notifyStop: make(chan struct{}),
	}
}

// SetDB mengatur koneksi database untuk notification loop
func (h *Hub) SetDB(db *gorm.DB) {
	h.db = db
}

// Run memulai loop utama hub untuk mengelola client dan pesan
func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			h.clients[c] = true
			h.mu.Unlock()
			h.logger.Info("WebSocket client terhubung", zap.Int("total_clients", len(h.clients)))

		case c := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
			}
			h.mu.Unlock()
			h.logger.Info("WebSocket client terputus", zap.Int("total_clients", len(h.clients)))

		case message := <-h.broadcast:
			h.mu.RLock()
			for c := range h.clients {
				select {
				case c.send <- message:
				default:
					// Channel penuh, tutup koneksi
					close(c.send)
					delete(h.clients, c)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast mengirimkan event ke semua client yang terhubung
func (h *Hub) Broadcast(event *Event) {
	data, err := json.Marshal(event)
	if err != nil {
		h.logger.Error("Gagal marshal event WebSocket", zap.Error(err))
		return
	}
	h.broadcast <- data
}

// HandleConnection menangani satu koneksi WebSocket baru
func (h *Hub) HandleConnection(conn *websocket.Conn) {
	c := &client{
		conn:   conn,
		send:   make(chan []byte, 256),
		logger: h.logger,
	}

	h.register <- c

	// Goroutine untuk menulis pesan ke client
	go func() {
		defer func() {
			h.unregister <- c
			conn.Close()
		}()

		for {
			message, ok := <-c.send
			if !ok {
				// Channel ditutup
				_ = conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				h.logger.Warn("Gagal menulis ke WebSocket", zap.Error(err))
				return
			}
		}
	}()

	// Baca pesan dari client (agar koneksi tetap hidup dan handle ping)
	for {
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				h.logger.Warn("WebSocket ditutup tidak terduga", zap.Error(err))
			}
			break
		}

		// Handle ping dari client
		if msgType == websocket.TextMessage {
			var incoming Event
			if err := json.Unmarshal(msg, &incoming); err == nil && incoming.Type == EventPing {
				pong := NewEvent(EventPing, map[string]string{"status": "pong"})
				if data, err := json.Marshal(pong); err == nil {
					c.send <- data
				}
			}
		}
	}
}

// StartNotificationLoop memulai goroutine untuk mengirim notifikasi secara periodik
func (h *Hub) StartNotificationLoop() {
	if h.db == nil {
		h.logger.Warn("Notification loop: db not set")
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
		h.logger.Info("Notification loop started")
		// Ticker 1 detik — cek setiap detik, kirim sesuai LoopInterval masing-masing notifikasi
		// (interval ticker harus < interval notifikasi terkecil agar tidak miss)
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		lastSent := make(map[uuid.UUID]time.Time)

		for {
			select {
			case <-h.notifyStop:
				h.logger.Info("Notification loop stopped")
				return
			case tick := <-ticker.C:
				if h.db == nil {
					continue
				}

				var notifications []entity.TvNotification
				h.db.Where("loop_enabled = ? AND is_active = ?", true, true).Find(&notifications)

				for _, n := range notifications {
					last, exists := lastSent[n.ID]
					interval := time.Duration(n.LoopInterval) * time.Second
					if interval < 5*time.Second {
						interval = 5 * time.Second
					}

					// Kirim jika belum pernah dikirim atau sudah melewati interval
					if !exists || tick.Sub(last) >= interval {
						evt := NewEvent(EventTVNotification, map[string]interface{}{
							"id":                 n.ID,
							"title":              n.Title,
							"message":            n.Message,
							"imageUrl":           n.ImageURL,
							"priority":           n.Priority,
							"targetAll":          n.TargetAll,
							"targetConsoleIds":   n.TargetConsoleIDs,
							"activeSessionsOnly": n.ActiveSessionsOnly,
						})
						h.Broadcast(evt)
						lastSent[n.ID] = tick
						h.logger.Info("Notification broadcast", zap.String("title", n.Title), zap.Duration("interval", interval))
					}
				}
			}
		}
	}()
}

// StopNotificationLoop menghentikan goroutine notifikasi
func (h *Hub) StopNotificationLoop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.notifyRunning {
		close(h.notifyStop)
		h.notifyRunning = false
	}
}

// IsNotificationRunning mengecek status loop
func (h *Hub) IsNotificationRunning() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.notifyRunning
}

// BroadcastToConsole mengirim event ke semua client (kontrol TV spesifik)
func (h *Hub) BroadcastToConsole(consoleID uuid.UUID, event *Event) {
	h.Broadcast(event)
}
// StartAutoStop memulai goroutine auto-stop sesi yang expired
func (h *Hub) StartAutoStop() {
	if h.db == nil {
		h.logger.Warn("AutoStop: db not set")
		return
	}

	go func() {
		h.logger.Info("AutoStop loop started (check every 30s)")
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			if h.db == nil {
				continue
			}

			// Cari sesi aktif yang end_scheduled_at sudah lewat
			var expiredSessions []struct {
				ID        uuid.UUID `gorm:"column:id"`
				ConsoleID uuid.UUID `gorm:"column:console_id"`
			}
			h.db.Raw(`
				SELECT id, console_id FROM sessions
				WHERE status = 'active'
				  AND end_scheduled_at IS NOT NULL
				  AND end_scheduled_at < NOW()
			`).Scan(&expiredSessions)

			for _, s := range expiredSessions {
				// Panggil byoneEndSession untuk setiap sesi expired
				if err := h.db.Exec(`SELECT "byoneEndSession"(?)`, s.ID).Error; err != nil {
					h.logger.Warn("AutoStop: gagal mengakhiri sesi", zap.String("session_id", s.ID.String()), zap.Error(err))
					continue
				}

				// Broadcast event ke client
				h.Broadcast(NewEvent(EventSessionEnded, map[string]interface{}{
					"sessionId": s.ID,
					"consoleId": s.ConsoleID,
					"reason":    "auto_stop",
				}))

				// Kirim sleep ke TV
				h.Broadcast(NewEvent(EventTVSleep, map[string]interface{}{
					"consoleId": s.ConsoleID,
				}))

				h.logger.Info("AutoStop: sesi diakhiri otomatis", zap.String("session_id", s.ID.String()))
			}
		}
	}()
}