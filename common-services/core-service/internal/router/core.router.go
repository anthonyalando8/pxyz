package router

import (
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/redis/go-redis/v9"

	"core-service/internal/handler/rest"
	"x/shared/auth/middleware"
)

func SetupRoutes(
	r chi.Router,
	h *hrest.CountryHandler,
	auth *middleware.MiddlewareWithClient,
	rdb *redis.Client,
) chi.Router {
	// ---- Global Middleware ----
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"}, // allow all origins
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "ngrok-skip-browser-warning",},
		ExposedHeaders:   []string{"Link"},
		//AllowCredentials: true,
		AllowCredentials: false, // must be false when using "*"
		MaxAge:           300,
	}))

	// Global rate limit
	r.Use(auth.RateLimit(rdb, 100, time.Minute, 10*time.Minute, "global"))

	uploadDir := "/app/uploads"
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		os.MkdirAll(uploadDir, 0755)
	}

	r.Group(func(r chi.Router) {
		r.Get("/core/countries", h.HandleGetCountries)          // List all countries
	})

	// ============================================================
	// Protected Endpoints (require auth)
	// ============================================================
	r.Group(func(pr chi.Router) {
		pr.Use(auth.Middleware)
		// Country management (admin/internal use cases)
		pr.Post("/core/countries/refresh", h.HandleRefreshCountries) // refresh from external source
	})

	return r
}
