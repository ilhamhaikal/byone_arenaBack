package websocket

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
	"go.uber.org/zap"
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
}

// NewHub membuat instance baru Hub
func NewHub(logger *zap.Logger) *Hub {
	return &Hub{
		clients:    make(map[*client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *client),
		unregister: make(chan *client),
		logger:     logger,
	}
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
