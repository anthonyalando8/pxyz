// internal/handler/p2p_websocket_handler.go
package handler

import (
	"encoding/json"

	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)


// readPump pumps messages from the WebSocket connection to the handler
func (c *Client) readPump() {
	defer func() {
		c.Handler.unregisterClient(c)
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
				c.Handler.logger.Error("WebSocket error",
					zap.String("client_id", c.ID),
					zap.Error(err))
			}
			break
		}

		c.Handler.handleMessage(c, message)
	}
}

// writePump pumps messages from the handler to the WebSocket connection
func (c *Client) writePump() {
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

			// Add queued messages to current message
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

// SendJSON sends a JSON message to the client
func (c *Client) SendJSON(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		c.Handler.logger.Error("Failed to marshal JSON",
			zap.String("client_id", c.ID),
			zap.Error(err))
		return
	}

	select {
	case c.Send <- data:
	default:
		c.Handler.logger.Warn("Client send buffer full",
			zap.String("client_id", c.ID))
	}
}

// SendError sends an error message to the client
func (c *Client) SendError(message string) {
	c.SendJSON(&WSResponse{
		Type:    "error",
		Success: false,
		Error:   message,
	})
}