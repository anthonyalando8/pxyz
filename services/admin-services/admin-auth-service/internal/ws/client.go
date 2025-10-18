package ws

import (
	"github.com/gorilla/websocket"
)

type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan Message
	UserID string
}

func NewClient(userID string, conn *websocket.Conn, hub *Hub) *Client {
    return &Client{
        hub:    hub,
        conn:   conn,
        send:   make(chan Message, 256),
        UserID: userID,
    }
}

func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (c *Client) WritePump() {
    defer c.conn.Close()

    for message := range c.send {
        if err := c.conn.WriteJSON(message); err != nil {
            return
        }
    }
    // channel closed -> close socket
    c.conn.WriteMessage(websocket.CloseMessage, []byte{})
}
