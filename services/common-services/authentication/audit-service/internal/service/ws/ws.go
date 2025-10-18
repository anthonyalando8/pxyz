// security_audit_websocket.go
package websocket

import (
	"audit-service/internal/domain"
	"audit-service/internal/service/audit"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ================================
// WEBSOCKET MESSAGE TYPES
// ================================

const (
	MessageTypeSuspiciousActivity = "suspicious_activity"
	MessageTypeCriticalEvent      = "critical_event"
	MessageTypeHighRiskUser       = "high_risk_user"
	MessageTypeAccountLocked      = "account_locked"
	MessageTypeRateLimitExceeded  = "rate_limit_exceeded"
)

type WebSocketMessage struct {
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      interface{}            `json:"data"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type SuspiciousActivityNotification struct {
	UserID       string    `json:"user_id"`
	ActivityType string    `json:"activity_type"`
	RiskScore    int       `json:"risk_score"`
	IPAddress    string    `json:"ip_address"`
	Activities   []string  `json:"activities"`
	Timestamp    time.Time `json:"timestamp"`
}

type CriticalEventNotification struct {
	EventType   string    `json:"event_type"`
	UserID      *string   `json:"user_id,omitempty"`
	Severity    string    `json:"severity"`
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp"`
}

type HighRiskUserNotification struct {
	UserID    string    `json:"user_id"`
	RiskScore int       `json:"risk_score"`
	Reason    string    `json:"reason"`
	Timestamp time.Time `json:"timestamp"`
}

type AccountLockedNotification struct {
	UserID    string     `json:"user_id"`
	Reason    string     `json:"reason"`
	UnlockAt  *time.Time `json:"unlock_at,omitempty"`
	Timestamp time.Time  `json:"timestamp"`
}

// ================================
// WEBSOCKET CLIENT
// ================================

type Client struct {
	ID      string
	UserID  *string // For user-specific notifications
	IsAdmin bool    // For admin dashboard notifications
	Conn    *websocket.Conn
	Send    chan []byte
	Hub     *Hub
	mu      sync.Mutex
}

func (c *Client) ReadPump() {
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
		_, _, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
	}
}

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

			// Add queued messages
			n := len(c.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.Send)
			}

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

// ================================
// WEBSOCKET HUB
// ================================

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.Send)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// BroadcastToUser sends a message to a specific user
func (h *Hub) BroadcastToUser(userID string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if client.UserID != nil && *client.UserID == userID {
			select {
			case client.Send <- message:
			default:
				close(client.Send)
				delete(h.clients, client)
			}
		}
	}
}

// BroadcastToAdmins sends a message to all admin clients
func (h *Hub) BroadcastToAdmins(message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if client.IsAdmin {
			select {
			case client.Send <- message:
			default:
				close(client.Send)
				delete(h.clients, client)
			}
		}
	}
}

// ================================
// SECURITY AUDIT NOTIFIER
// ================================

type SecurityAuditNotifier struct {
	hub          *Hub
	auditService *service.SecurityAuditService
}

func NewSecurityAuditNotifier(hub *Hub, auditService *service.SecurityAuditService) *SecurityAuditNotifier {
	return &SecurityAuditNotifier{
		hub:          hub,
		auditService: auditService,
	}
}

// NotifySuspiciousActivity sends real-time notification of suspicious activity
func (n *SecurityAuditNotifier) NotifySuspiciousActivity(ctx context.Context, userID, ipAddress string, activities []string, riskScore int) error {
	notification := SuspiciousActivityNotification{
		UserID:       userID,
		ActivityType: "multiple",
		RiskScore:    riskScore,
		IPAddress:    ipAddress,
		Activities:   activities,
		Timestamp:    time.Now(),
	}

	msg := WebSocketMessage{
		Type:      MessageTypeSuspiciousActivity,
		Timestamp: time.Now(),
		Data:      notification,
		Metadata: map[string]interface{}{
			"risk_level": getRiskLevel(riskScore),
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// Send to the specific user
	n.hub.BroadcastToUser(userID, data)

	// If high risk, also notify admins
	if riskScore >= 70 {
		n.hub.BroadcastToAdmins(data)
	}

	return nil
}

// NotifyCriticalEvent sends real-time notification of critical security events
func (n *SecurityAuditNotifier) NotifyCriticalEvent(ctx context.Context, event *domain.SecurityAuditLog) error {
	notification := CriticalEventNotification{
		EventType: event.EventType,
		UserID:    event.UserID,
		Severity:  event.Severity,
		Timestamp: event.CreatedAt,
	}

	if event.Description != nil {
		notification.Description = *event.Description
	}

	msg := WebSocketMessage{
		Type:      MessageTypeCriticalEvent,
		Timestamp: time.Now(),
		Data:      notification,
		Metadata: map[string]interface{}{
			"event_category": event.EventCategory,
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// Critical events go to admins
	n.hub.BroadcastToAdmins(data)

	return nil
}

// NotifyHighRiskUsers sends periodic updates about high-risk users
func (n *SecurityAuditNotifier) NotifyHighRiskUsers(ctx context.Context) error {
	activities, err := n.auditService.GetHighRiskUsers(ctx, 70, 10)
	if err != nil {
		return err
	}

	for _, activity := range activities {
		notification := HighRiskUserNotification{
			UserID:    activity.UserID,
			RiskScore: activity.RiskScore,
			Reason:    activity.ActivityType,
			Timestamp: time.Now(),
		}

		msg := WebSocketMessage{
			Type:      MessageTypeHighRiskUser,
			Timestamp: time.Now(),
			Data:      notification,
		}

		data, err := json.Marshal(msg)
		if err != nil {
			continue
		}

		n.hub.BroadcastToAdmins(data)
	}

	return nil
}

// NotifyAccountLocked sends notification when an account is locked
func (n *SecurityAuditNotifier) NotifyAccountLocked(ctx context.Context, userID, reason string, unlockAt *time.Time) error {
	notification := AccountLockedNotification{
		UserID:    userID,
		Reason:    reason,
		UnlockAt:  unlockAt,
		Timestamp: time.Now(),
	}

	msg := WebSocketMessage{
		Type:      MessageTypeAccountLocked,
		Timestamp: time.Now(),
		Data:      notification,
		Metadata: map[string]interface{}{
			"severity": "critical",
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// Notify the user
	n.hub.BroadcastToUser(userID, data)

	// Also notify admins
	n.hub.BroadcastToAdmins(data)

	return nil
}

// NotifyRateLimitExceeded sends notification when rate limit is exceeded
func (n *SecurityAuditNotifier) NotifyRateLimitExceeded(ctx context.Context, identifier, ipAddress string) error {
	msg := WebSocketMessage{
		Type:      MessageTypeRateLimitExceeded,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"identifier": identifier,
			"ip_address": ipAddress,
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// Notify admins only
	n.hub.BroadcastToAdmins(data)

	return nil
}

// StartPeriodicNotifications starts background workers for periodic notifications
func (n *SecurityAuditNotifier) StartPeriodicNotifications(ctx context.Context) {
	// High-risk users notification every 5 minutes
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := n.NotifyHighRiskUsers(ctx); err != nil {
				log.Printf("Failed to notify high-risk users: %v", err)
			}

		case <-ctx.Done():
			return
		}
	}
}

// ================================
// HELPER FUNCTIONS
// ================================

func getRiskLevel(score int) string {
	switch {
	case score >= 80:
		return "critical"
	case score >= 60:
		return "high"
	case score >= 40:
		return "medium"
	default:
		return "low"
	}
}

// ================================
// HTTP HANDLER FOR WEBSOCKET
// ================================

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// TODO: Implement proper origin checking in production
		return true
	},
}

func ServeWebSocket(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	// Extract user info from context (set by auth middleware)
	userID := getUserIDFromContext(r.Context())
	isAdmin := isAdminFromContext(r.Context())

	client := &Client{
		ID:      generateClientID(),
		UserID:  userID,
		IsAdmin: isAdmin,
		Conn:    conn,
		Send:    make(chan []byte, 256),
		Hub:     hub,
	}

	client.Hub.register <- client

	// Start goroutines
	go client.WritePump()
	go client.ReadPump()
}

func getUserIDFromContext(ctx context.Context) *string {
	if userID, ok := ctx.Value("user_id").(string); ok {
		return &userID
	}
	return nil
}

func isAdminFromContext(ctx context.Context) bool {
	if isAdmin, ok := ctx.Value("is_admin").(bool); ok {
		return isAdmin
	}
	return false
}

func generateClientID() string {
	return fmt.Sprintf("client_%d", time.Now().UnixNano())
}
