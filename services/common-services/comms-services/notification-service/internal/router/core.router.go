package router

import (
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/redis/go-redis/v9"

	hrest "notification-service/internal/handler/http"
	wshandler "notification-service/internal/handler/ws"
	"x/shared/auth/middleware"
)

// SetupRoutes configures the HTTP routes for the notification service
func SetupRoutes(
	r chi.Router,
	h *hrest.NotificationHandler,
	wsHandler *wshandler.WSHandler,
	auth *middleware.MiddlewareWithClient,
	rdb *redis.Client,
) chi.Router {
	// ---- Global Middleware ----
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders: []string{
			"Accept",
			"Authorization",
			"Content-Type",
			"X-CSRF-Token",
			"ngrok-skip-browser-warning",
		},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Use(auth.RateLimit(rdb, 100, time.Minute, 10*time.Minute, "global"))

	// Ensure upload dir exists
	uploadDir := "/app/uploads"
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		_ = os.MkdirAll(uploadDir, 0755)
	}

	// ============================================================
	// Notifications Routes (all require auth)
	// ============================================================
	r.Route("/api/v1/user/notifications", func(r chi.Router) {
		r.Use(auth.Middleware)

		// Notification CRUD & prefs
		r.Get("/", h.ListNotifications)
		r.Get("/unread", h.ListUnread)
		r.Get("/unread/count", h.CountUnread)
		r.Patch("/{id}/read", h.MarkAsRead)
		r.Patch("/{id}/hide", h.HideNotification)

		r.Get("/preferences", h.GetPreference)
		r.Post("/preferences", h.UpsertPreference)
		r.Delete("/preferences", h.DeletePreference)

		// WebSocket endpoint
		r.Get("/ws", wsHandler.HandleNotifications)
	})
	return r
}