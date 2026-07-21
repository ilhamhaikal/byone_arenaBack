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
}

func NewEvent(eventType EventType, payload interface{}) *Event {
	return &Event{Type: eventType, Payload: payload, Timestamp: time.Now()}
}

type client struct {
	conn   *websocket.Conn
	send   chan []byte
	logger *zap.Logger
}

type Hub struct {
	mu            sync.RWMutex
	clients       map[*client]bool
	broadcast     chan []byte
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
		broadcast:  make(chan []byte, 256),
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
				select {
				case c.send <- message:
				default:
					close(c.send)
					delete(h.clients, c)
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) Broadcast(event *Event) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	h.broadcast <- data
}

func (h *Hub) HandleConnection(conn *websocket.Conn) {
	c := &client{conn: conn, send: make(chan []byte, 256), logger: h.logger}
	h.register <- c

	go func() {
		defer func() {
			h.unregister <- c
			conn.Close()
		}()
		for message := range c.send {
			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
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
					c.send <- data
				}
			}
		}
	}
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
				h.db.Where("loop_enabled = ? AND is_active = ?", true, true).Find(&notifications)
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
						})
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
			// Safety net: reset konsol stuck in_use tanpa sesi aktif
			h.db.Exec(`
				UPDATE consoles SET status = 'available', updated_at = NOW()
				WHERE status = 'in_use'
				  AND NOT EXISTS (
				    SELECT 1 FROM sessions s
				    WHERE s.console_id = consoles.id AND s.status = 'active'
				  )
			`)
			// Safety net: reset konsol stuck rented_out tanpa rental aktif
			h.db.Exec(`
				UPDATE consoles SET status = 'available', updated_at = NOW()
				WHERE status = 'rented_out'
				  AND NOT EXISTS (
				    SELECT 1 FROM daily_rentals d
				    WHERE d.console_id = consoles.id AND d.status = 'active'
				  )
			`)
			// Overdue daily rentals
			h.db.Exec(`
				UPDATE daily_rentals SET status = 'overdue', updated_at = NOW()
				WHERE status = 'active' AND end_date < CURRENT_DATE
			`)
			// Cari sesi expired
			var expiredSessions []struct {
				ID        uuid.UUID `gorm:"column:id"`
				ConsoleID uuid.UUID `gorm:"column:console_id"`
			}
			h.db.Raw(`
				SELECT id, console_id FROM sessions
				WHERE status = 'active' AND end_scheduled_at IS NOT NULL AND end_scheduled_at < NOW()
			`).Scan(&expiredSessions)
			for _, s := range expiredSessions {
				h.db.Exec(`SELECT "byoneEndSession"(?)`, s.ID)
				h.Broadcast(NewEvent(EventSessionEnded, map[string]interface{}{
					"sessionId": s.ID, "consoleId": s.ConsoleID, "reason": "auto_stop",
				}))
				h.Broadcast(NewEvent(EventTVSleep, map[string]interface{}{
					"consoleId": s.ConsoleID,
				}))
			}
		}
	}()
}
