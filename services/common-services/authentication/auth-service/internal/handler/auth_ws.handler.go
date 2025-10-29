package handler

import (
	"net/http"

	"auth-service/internal/ws"
	"x/shared/auth/middleware"
)

type WSHandler struct {
	server *ws.Server
}

func NewWSHandler(server *ws.Server) *WSHandler {
	return &WSHandler{server: server}
}

func (h *WSHandler) HandleWS(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.ContextUserID).(string)
	deviceID := r.Context().Value(middleware.ContextDeviceID).(string)
	h.server.ServeWS(w, r, userID, deviceID)
}