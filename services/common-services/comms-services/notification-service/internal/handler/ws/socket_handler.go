package wshandler

import (
	"net/http"
	"time"
	"log"

	"github.com/gorilla/websocket"
	"notification-service/pkg/notifier/ws"
	"x/shared/auth/middleware"
)

type WSHandler struct {
	manager *ws.Manager
}

func NewWSHandler(manager *ws.Manager) *WSHandler {
	return &WSHandler{manager: manager}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// 🔒 TODO: tighten with allowed origins if needed
		return true
	},
}

// HandleNotifications upgrades HTTP -> WebSocket and registers connection
func (h *WSHandler) HandleNotifications(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextUserID).(string)
	userType, _ := r.Context().Value(middleware.ContextUserType).(string)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "upgrade failed", http.StatusBadRequest)
		return
	}

	log.Printf("[NOTIFICATION][WS] userID=%s userType=%s", userID, userType)


	c := h.manager.Add(userType, userID, conn)

	// Reader loop: listen for pongs and client messages
	conn.SetReadLimit(512)
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		c.LastSeen = time.Now()
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}

	// Cleanup when connection closes
	h.manager.Remove(c)
}
