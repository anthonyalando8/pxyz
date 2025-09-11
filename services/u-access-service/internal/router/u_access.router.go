package router

import (
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/redis/go-redis/v9"

	hrest "u-rbac-service/internal/handler/rest"
	"x/shared/auth/middleware"
)

func SetupRoutes(
	r chi.Router,
	h *hrest.ModuleHandler,
	auth *middleware.MiddlewareWithClient,
	rdb *redis.Client,
) chi.Router {
	// ---- Global Middleware ----
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://127.0.0.1:5500", "http://localhost:5500", "https://4bcbc3ea8466.ngrok-free.app"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Requested-With"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Global rate limit
	r.Use(auth.RateLimit(rdb, 100, time.Minute, 10*time.Minute, "global"))

	uploadDir := "/app/uploads"
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		os.MkdirAll(uploadDir, 0755)
	}


	// ============================================================
	// Protected Endpoints (require auth)
	// ============================================================
	r.Group(func(pr chi.Router) {
		pr.Use(auth.Middleware)

		// Module management
		pr.Post("/urbac/modules/create", h.HandleCreateModule)    // Create new module
		pr.Get("/urbac/user/permission", h.HandleGetUserPermissions) // Update module by code
	})

	return r
}
