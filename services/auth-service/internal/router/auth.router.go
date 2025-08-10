package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/redis/go-redis/v9"

	"auth-service/internal/handler"
	"x/shared/auth/middleware"
)

func SetupRoutes(r chi.Router, h *handler.AuthHandler, auth *middleware.MiddlewareWithClient, wsHandler *handler.WSHandler, rdb *redis.Client) chi.Router {
	
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://127.0.0.1:5500", "http://localhost:5500"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Post("/auth/exists", h.HandleUserExists)
	r.Post("/auth/register", h.HandleRegister)
	r.Post("/auth/login", h.HandleLogin)

	r.Group(func(pr chi.Router) {
		pr.Use(auth.Middleware)

		// WebSocket endpoint (protected)
		pr.Get("/auth/ws", wsHandler.HandleWS)

		pr.Patch("/auth/email", h.HandleChangeEmail)
		pr.Patch("/auth/password", h.HandleChangePassword)
		pr.Patch("/auth/name", h.HandleUpdateName)

		pr.Delete("/auth/logout", h.LogoutHandler(auth.Client))
		pr.Delete("/auth/sessions", h.LogoutAllHandler(auth.Client, rdb))
		pr.Delete("/auth/sessions/{id}", h.DeleteSessionByIDHandler(auth.Client))

		pr.Get("/auth/sessions", h.ListSessionsHandler(auth.Client))
		pr.Get("/auth/profile", h.HandleProfile)
	})
	return r
}