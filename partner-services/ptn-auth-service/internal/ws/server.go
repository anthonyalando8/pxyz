package ws

import (
	"net/http"

	"github.com/gorilla/websocket"
)

type Server struct {
	hub *Hub
}

func NewServer() *Server {
	return &Server{hub: NewHub()}
}

func (s *Server) Start() {
	go s.hub.Run()
}

func (s *Server) Hub() *Hub {
    return s.hub
}

func (s *Server) ServeWS(w http.ResponseWriter, r *http.Request, userID string) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Failed to upgrade connection", http.StatusInternalServerError)
		return
	}

	client := NewClient(userID, conn, s.hub)
	s.hub.register <- client

	go client.WritePump()
	go client.ReadPump()
}
