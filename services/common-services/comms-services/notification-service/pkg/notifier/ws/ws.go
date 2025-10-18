package ws

import (
	"log"
	"notification-service/internal/domain"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Connection wraps websocket.Conn with metadata
type Connection struct {
	Conn     *websocket.Conn
	UserKey  string
	LastSeen time.Time
}

type Manager struct {
	mu          sync.RWMutex
	connections map[string]map[*Connection]struct{} // userKey -> set of connections
}

func NewManager() *Manager {
	return &Manager{
		connections: make(map[string]map[*Connection]struct{}),
	}
}

func makeUserKey(userType, userID string) string {
	return userType + "_" + userID
}

// Add registers a connection for a user
func (m *Manager) Add(userType, userID string, conn *websocket.Conn) *Connection {
	userKey := makeUserKey(userType, userID)
	c := &Connection{Conn: conn, UserKey: userKey, LastSeen: time.Now()}

	m.mu.Lock()
	if _, ok := m.connections[userKey]; !ok {
		m.connections[userKey] = make(map[*Connection]struct{})
	}
	m.connections[userKey][c] = struct{}{}
	m.mu.Unlock()

	log.Printf("WS connected: %s (total=%d)", userKey, len(m.connections[userKey]))
	return c
}

// Remove disconnects and removes a connection
func (m *Manager) Remove(c *Connection) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if conns, ok := m.connections[c.UserKey]; ok {
		delete(conns, c)
		if len(conns) == 0 {
			delete(m.connections, c.UserKey)
		}
	}
	_ = c.Conn.Close()
	log.Printf("WS disconnected: %s", c.UserKey)
}

// Send sends a JSON message to all connections of a user
func (m *Manager) Send(userType, userID string, msg *domain.Message) {
    userKey := makeUserKey(userType, userID)

    wsMsg := domain.WSMessage{
        OwnerID:   msg.OwnerID,
        OwnerType: msg.OwnerType,
        Title:     msg.Title,
        Body:      msg.Body,
        Metadata:  msg.Metadata,
        Data:      msg.Data,
    }

    m.mu.RLock()
    defer m.mu.RUnlock()

    if conns, ok := m.connections[userKey]; ok {
        for c := range conns {
            if err := c.Conn.WriteJSON(wsMsg); err != nil {
                log.Printf("⚠️ failed WS send to %s: %v", userKey, err)
                go m.Remove(c)
            }
        }
    }
}


// Broadcast sends to all users
func (m *Manager) Broadcast(message interface{}) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, conns := range m.connections {
		for c := range conns {
			err := c.Conn.WriteJSON(message)
			if err != nil {
				log.Printf("⚠️ failed WS broadcast: %v", err)
				go m.Remove(c)
			}
		}
	}
}

// Heartbeat pings all connections periodically to keep them alive
func (m *Manager) Heartbeat(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		m.mu.RLock()
		for _, conns := range m.connections {
			for c := range conns {
				if time.Since(c.LastSeen) > 2*interval {
					go m.Remove(c)
					continue
				}
				_ = c.Conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(time.Second))
			}
		}
		m.mu.RUnlock()
	}
}
