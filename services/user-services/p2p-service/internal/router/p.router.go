// internal/router/router.go
package router

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/redis/go-redis/v9"

	wsh "p2p-service/internal/handler/websocket"
	rh "p2p-service/internal/handler/rest"

	"x/shared/auth/middleware"
)

func SetupRoutes(
	r chi.Router,
	p2pRestHandler *rh.P2PRestHandler,
	p2pWSHandler *wsh.P2PWebSocketHandler,
	auth *middleware.MiddlewareWithClient,
	rdb *redis.Client,
) chi.Router {
	// ---- Global Middleware ----
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "ngrok-skip-browser-warning"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Global rate limiting
	r.Use(auth.RateLimit(rdb, 100, time.Minute, 10*time.Minute, "global"))

	// ============================================================
	// Public Endpoints (Health Check)
	// ============================================================
	r.Group(func(pub chi.Router) {
		pub.Get("/p2p/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("P2P service is running"))
		})
	})

	// ============================================================
	// Authenticated P2P Endpoints
	// ============================================================
	r.Route("/api/p2p", func(pr chi.Router) {
		// Apply authentication middleware
		pr.Use(auth.Require([]string{"main"}, nil, nil))

		// ============ PROFILE MANAGEMENT (REST) ============
		pr.Route("/profile", func(profile chi.Router) {
			// Check if user has P2P profile
			profile.Get("/check", p2pRestHandler.CheckProfile)
			
			// Create P2P profile (with consent)
			profile.Post("/create", p2pRestHandler.CreateProfile)
			
			// Get user's own profile
			profile.Get("/", p2pRestHandler.GetProfile)
			
			// Update user's own profile
			profile.Put("/", p2pRestHandler.UpdateProfile)
		})

		// ============ WEBSOCKET CONNECTION ============
		// User must have created profile before connecting
		pr.Get("/ws", p2pWSHandler.HandleConnection)
	})

	return r
}