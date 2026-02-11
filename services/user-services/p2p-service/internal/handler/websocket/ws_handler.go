// internal/handler/p2p_websocket_handler.go
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"p2p-service/internal/usecase"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // TODO: Implement proper origin checking
	},
}

// P2PWebSocketHandler handles P2P WebSocket connections
type P2PWebSocketHandler struct {
	profileUsecase *usecase.P2PProfileUsecase
	// TODO: Add other usecases as they're implemented
	// adUsecase      *usecase.P2PAdUsecase
	// orderUsecase   *usecase.P2POrderUsecase
	// chatUsecase    *usecase.P2PChatUsecase
	
	logger *zap.Logger
	
	// Connection management
	clients   map[string]*Client
	clientsMu sync.RWMutex
}

// Client represents a WebSocket client connection
type Client struct {
	ID        string
	UserID    string
	ProfileID int64
	Conn      *websocket.Conn
	Send      chan []byte
	Handler   *P2PWebSocketHandler
}

// WSMessage represents a WebSocket message structure
type WSMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// WSResponse represents a WebSocket response structure
type WSResponse struct {
	Type    string      `json:"type"`
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
}

func NewP2PWebSocketHandler(
	profileUsecase *usecase.P2PProfileUsecase,
	logger *zap.Logger,
) *P2PWebSocketHandler {
	return &P2PWebSocketHandler{
		profileUsecase: profileUsecase,
		logger:         logger,
		clients:        make(map[string]*Client),
	}
}


// HandleConnection handles new WebSocket connections
func (h *P2PWebSocketHandler) HandleConnection(w http.ResponseWriter, r *http.Request) {
	// Extract user info from context (set by auth middleware)
	userID, ok := r.Context().Value("user_id").(string)
	if !ok || userID == "" {
		h.logger.Warn("Unauthorized WebSocket connection attempt")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Check if profile exists (DO NOT auto-create)
	profile, err := h.profileUsecase.GetProfileByUserID(r.Context(), userID)
	if err != nil {
		h.logger.Warn("User attempted to connect without P2P profile",
			zap.String("user_id", userID),
			zap.Error(err))
		
		// Send error response before closing
		conn, upgradeErr := upgrader.Upgrade(w, r, nil)
		if upgradeErr != nil {
			http.Error(w, "Failed to upgrade connection", http.StatusInternalServerError)
			return
		}
		
		// Send error message
		errorMsg := map[string]interface{}{
			"type":    "connection_error",
			"success": false,
			"error":   "NOT_JOINED_P2P",
			"message": "You have not joined P2P trading yet. Please create a P2P profile first.",
		}
		
		msgBytes, _ := json.Marshal(errorMsg)
		conn.WriteMessage(websocket.TextMessage, msgBytes)
		
		// Close connection after sending error
		time.Sleep(100 * time.Millisecond) // Give time for message to send
		conn.Close()
		return
	}

	//  Check if profile has given consent
	if !profile.HasConsent {
		h.logger.Warn("User attempted to connect without consent",
			zap.String("user_id", userID),
			zap.Int64("profile_id", profile.ID))
		
		conn, upgradeErr := upgrader.Upgrade(w, r, nil)
		if upgradeErr != nil {
			http.Error(w, "Failed to upgrade connection", http.StatusInternalServerError)
			return
		}
		
		errorMsg := map[string]interface{}{
			"type":    "connection_error",
			"success": false,
			"error":   "CONSENT_REQUIRED",
			"message": "You must accept the P2P trading terms and conditions before connecting.",
		}
		
		msgBytes, _ := json.Marshal(errorMsg)
		conn.WriteMessage(websocket.TextMessage, msgBytes)
		
		time.Sleep(100 * time.Millisecond)
		conn.Close()
		return
	}

	// Upgrade connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade WebSocket connection",
			zap.String("user_id", userID),
			zap.Error(err))
		return
	}

	// Create client
	client := &Client{
		ID:        fmt.Sprintf("%s_%d", userID, time.Now().Unix()),
		UserID:    userID,
		ProfileID: profile.ID,
		Conn:      conn,
		Send:      make(chan []byte, 256),
		Handler:   h,
	}

	// Register client
	h.registerClient(client)

	// Send welcome message
	client.SendJSON(&WSResponse{
		Type:    "connected",
		Success: true,
		Data: map[string]interface{}{
			"profile_id": profile.ID,
			"user_id":    userID,
			"username":   profile.Username,
		},
		Message: "Connected to P2P service",
	})

	// Start client goroutines
	go client.writePump()
	go client.readPump()

	h.logger.Info("New P2P WebSocket connection",
		zap.String("client_id", client.ID),
		zap.String("user_id", userID),
		zap.Int64("profile_id", profile.ID))
}


// registerClient registers a new client
func (h *P2PWebSocketHandler) registerClient(client *Client) {
	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()
	
	// Close existing connection for this user if any
	if existingClient, exists := h.clients[client.UserID]; exists {
		existingClient.Conn.Close()
	}
	
	h.clients[client.UserID] = client
}

// unregisterClient unregisters a client
func (h *P2PWebSocketHandler) unregisterClient(client *Client) {
	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()
	
	if _, exists := h.clients[client.UserID]; exists {
		delete(h.clients, client.UserID)
		close(client.Send)
	}

	h.logger.Info("Client disconnected",
		zap.String("client_id", client.ID),
		zap.String("user_id", client.UserID))
}

// ============================================================================
// MESSAGE ROUTING
// ============================================================================

// handleMessage routes incoming messages to appropriate handlers
func (h *P2PWebSocketHandler) handleMessage(client *Client, data []byte) {
	var msg WSMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		h.logger.Error("Failed to parse WebSocket message",
			zap.String("client_id", client.ID),
			zap.Error(err))
		client.SendError("Invalid message format")
		return
	}

	h.logger.Debug("Received WebSocket message",
		zap.String("type", msg.Type),
		zap.String("client_id", client.ID))

	ctx := context.Background()

	// Route message based on type
	switch msg.Type {
	// ============ PROFILE OPERATIONS ============
	case "profile.get":
		h.handleGetProfile(ctx, client, msg.Data)
	case "profile.update":
		h.handleUpdateProfile(ctx, client, msg.Data)
	case "profile.stats":
		h.handleGetProfileStats(ctx, client, msg.Data)
	case "profile.search":
		h.handleSearchProfiles(ctx, client, msg.Data)

	// ============ AD OPERATIONS (TODO) ============
	case "ad.create":
		h.handleCreateAd(ctx, client, msg.Data)
	case "ad.list":
		h.handleListAds(ctx, client, msg.Data)
	case "ad.update":
		h.handleUpdateAd(ctx, client, msg.Data)
	case "ad.delete":
		h.handleDeleteAd(ctx, client, msg.Data)
	case "ad.my_ads":
		h.handleGetMyAds(ctx, client, msg.Data)

	// ============ ORDER OPERATIONS (TODO) ============
	case "order.create":
		h.handleCreateOrder(ctx, client, msg.Data)
	case "order.list":
		h.handleListOrders(ctx, client, msg.Data)
	case "order.get":
		h.handleGetOrder(ctx, client, msg.Data)
	case "order.cancel":
		h.handleCancelOrder(ctx, client, msg.Data)
	case "order.confirm_payment":
		h.handleConfirmPayment(ctx, client, msg.Data)
	case "order.release_crypto":
		h.handleReleaseCrypto(ctx, client, msg.Data)

	// ============ CHAT OPERATIONS (TODO) ============
	case "chat.send_message":
		h.handleSendMessage(ctx, client, msg.Data)
	case "chat.get_messages":
		h.handleGetMessages(ctx, client, msg.Data)
	case "chat.mark_read":
		h.handleMarkRead(ctx, client, msg.Data)

	// ============ DISPUTE OPERATIONS (TODO) ============
	case "dispute.raise":
		h.handleRaiseDispute(ctx, client, msg.Data)
	case "dispute.get":
		h.handleGetDispute(ctx, client, msg.Data)

	// ============ REVIEW OPERATIONS (TODO) ============
	case "review.create":
		h.handleCreateReview(ctx, client, msg.Data)
	case "review.list":
		h.handleListReviews(ctx, client, msg.Data)

	// ============ NOTIFICATION OPERATIONS (TODO) ============
	case "notification.list":
		h.handleListNotifications(ctx, client, msg.Data)
	case "notification.mark_read":
		h.handleMarkNotificationRead(ctx, client, msg.Data)

	default:
		client.SendError(fmt.Sprintf("Unknown message type: %s", msg.Type))
	}
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// BroadcastToUser sends a message to a specific user
func (h *P2PWebSocketHandler) BroadcastToUser(userID string, message interface{}) {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()

	if client, exists := h.clients[userID]; exists {
		client.SendJSON(message)
	}
}

// BroadcastToAll sends a message to all connected clients
func (h *P2PWebSocketHandler) BroadcastToAll(message interface{}) {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()

	for _, client := range h.clients {
		client.SendJSON(message)
	}
}

// GetConnectedClients returns the number of connected clients
func (h *P2PWebSocketHandler) GetConnectedClients() int {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()
	return len(h.clients)
}