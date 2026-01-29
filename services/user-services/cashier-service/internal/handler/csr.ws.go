// handler/websocket.go
package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"x/shared/auth/middleware"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Configure properly for production
	},
}

type Client struct {
	UserID string
	Conn   *websocket.Conn
	Send   chan []byte
	Hub    *Hub
}

type Hub struct {
	clients    map[string]*Client
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	mu         sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.UserID] = client
			h.mu.Unlock()
			log.Printf("[WebSocket] User %s connected", client.UserID)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.UserID]; ok {
				delete(h.clients, client.UserID)
				close(client.Send)
				log.Printf("[WebSocket] User %s disconnected", client.UserID)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for _, client := range h.clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(h.clients, client.UserID)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// GetClient retrieves a client by user ID
func (h *Hub) GetClient(userID string) (*Client, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	client, exists := h.clients[userID]
	return client, exists
}

// SendToUser sends a message to a specific user
func (h *Hub) SendToUser(userID string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if client, ok := h.clients[userID]; ok {
		select {
		case client.Send <- message:
		default:
			log.Printf("[WebSocket] Failed to send message to user %s", userID)
		}
	}
}

// SendJSON sends a JSON message directly (for event handlers)
func (c *Client) SendJSON(data interface{}) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		log.Printf("[WebSocket] Failed to marshal JSON: %v", err)
		return err
	}

	select {
	case c.Send <- bytes:
		return nil
	default:
		log.Printf("[WebSocket] Send channel full for user %s", c.UserID)
		return ErrChannelFull
	}
}

// SendMessage sends a structured message with type and data
func (c *Client) SendMessage(msgType string, data interface{}) {
	msg := WSResponse{
		Type:      msgType,
		Data:      data,
		Timestamp: time.Now().Unix(),
	}

	bytes, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[WebSocket] Failed to marshal message: %v", err)
		return
	}

	select {
	case c.Send <- bytes:
	default:
		log.Printf("[WebSocket] Send channel full for user %s", c.UserID)
	}
}

// SendError sends an error message
func (c *Client) SendError(message string) {
	c.SendMessage("error", map[string]string{"message": message})
}

// SendSuccess sends a success message
func (c *Client) SendSuccess(message string, data interface{}) {
	c.SendMessage("success", map[string]interface{}{
		"message": message,
		"data":    data,
	})
}

// ReadPump handles incoming WebSocket messages
func (c *Client) ReadPump(handler *PaymentHandler) {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[WebSocket] Error: %v", err)
			}
			break
		}

		// Parse incoming message
		var msg WSMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			c.SendError("invalid message format")
			continue
		}

		// Route message to appropriate handler
		handler.HandleWSMessage(c, &msg)
	}
}

// WritePump handles outgoing WebSocket messages
func (c *Client) WritePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// WebSocket message types
type WSMessage struct {
	Type string          `json:"type"` // deposit, withdraw, get_partners, get_accounts, get_history
	Data json.RawMessage `json:"data"`
}

type WSResponse struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp int64       `json:"timestamp"`
}

// Custom errors
var (
	ErrChannelFull = fmt.Errorf("send channel is full")
)

// WebSocket endpoint handler
func (h *PaymentHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WebSocket] Upgrade error: %v", err)
		return
	}

	client := &Client{
		UserID: userID,
		Conn:   conn,
		Send:   make(chan []byte, 256),
		Hub:    h.hub,
	}

	client.Hub.register <- client

	// Send welcome message
	client.SendMessage("connected", map[string]interface{}{
		"user_id":   userID,
		"timestamp": time.Now().Unix(),
	})

	// Start goroutines
	go client.WritePump()
	go client.ReadPump(h)
}
