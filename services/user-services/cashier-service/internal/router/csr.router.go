package router

import (
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/redis/go-redis/v9"

	"cashier-service/internal/handler"
	"x/shared/auth/middleware"
)

func SetupRoutes(
	r chi.Router,
	h *handler.PaymentHandler,
	auth *middleware.MiddlewareWithClient,
	rdb *redis.Client,
) chi. Router {
	// ---- Global Middleware ----
	r. Use(cors.Handler(cors. Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "ngrok-skip-browser-warning"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Global rate limiting
	r.Use(auth.RateLimit(rdb, 100, time.Minute, 10*time.Minute, "global"))

	uploadDir := "/app/uploads"
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		os.MkdirAll(uploadDir, 0755)
	}

	// ============================================================
	// Public Endpoints (Webhooks & Callbacks)
	// ============================================================
	r.Group(func(pub chi.Router) {
		pub.Get("/cashier/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		})

		// Partner deposit callback (when partner credits user)
		// pub.Post("/cashier/partner/deposit/callback", h.HandlePartnerDepositCallback)
	})

	// ============================================================
	// Authenticated Endpoints (User Portal)
	// ============================================================
	r.Route("/cashier/svc", func(pr chi.Router) {
		pr.Use(auth.Require([]string{"main"}, nil, /*[]string{"user"}*/ nil))

		// ---- WebSocket Endpoint ----
		pr.Get("/ws", h.HandleWebSocket)

		// ---- File Uploads ----
		pr.Handle("/uploads/*", http.StripPrefix("/cashier/svc/uploads/", http.FileServer(http.Dir(uploadDir))))

		// ---- Legacy HTTP Endpoints (Optional - can be deprecated) ----
		// pr.Post("/deposit", h.DepositHandler)
		// pr.Post("/withdraw", h.WithdrawHandler)
		// pr.Get("/accounts", h.GetUserAccountsHandler)
	})

	return r
}