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
	// P2P ROUTES
	// ============================================================
	r.Route("/p2p", func(pr chi.Router) {

		// ---------- Public ----------
		pr.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("P2P service is running"))
		})

		// ---------- Authenticated ----------
		pr.Group(func(authPr chi.Router) {
			authPr.Use(auth.Require([]string{"main"}, nil, nil))

			// ============ PROFILE MANAGEMENT ============
			authPr.Route("/profile", func(profile chi.Router) {
				profile.Get("/check", p2pRestHandler.CheckProfile)
				profile.Post("/create", p2pRestHandler.CreateProfile)
				profile.Get("/", p2pRestHandler.GetProfile)
				profile.Put("/", p2pRestHandler.UpdateProfile)
			})

			// ============ WEBSOCKET ============
			authPr.Get("/ws", p2pWSHandler.HandleConnection)
		})
	})

	return r
}
