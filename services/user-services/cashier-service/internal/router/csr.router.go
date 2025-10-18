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
) chi.Router {
	// ---- Global Middleware ----
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"}, // allow all origins
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "ngrok-skip-browser-warning"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false, // must be false when using "*"
		MaxAge:           300,
	}))

	// Global rate limiting
	r.Use(auth.RateLimit(rdb, 100, time.Minute, 10*time.Minute, "global"))

	uploadDir := "/app/uploads"
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		os.MkdirAll(uploadDir, 0755)
	}

	// ============================================================
	// Public Endpoints (Mpesa webhook/callbacks)
	// ============================================================
	r.Group(func(pr chi.Router) {
		pr.Post("/cashier/mpesa/callback", h.MpesaCallback)
	})

	// ============================================================
	// Authenticated Endpoints (User actions)
	// ============================================================
	r.Group(func(pr chi.Router) {
		pr.Use(auth.Require([]string{"main"}, nil, nil)) // <--- Updated to include roles (nil for now)

		// Uploads (proof of payment, receipts, etc.)
		pr.Handle("/cashier/uploads/*", http.StripPrefix("/cashier/uploads/", http.FileServer(http.Dir(uploadDir))))

		// Deposit & Withdraw (generic, provider decided in request body)
		pr.Post("/cashier/deposit", h.DepositHandler)
		pr.Post("/cashier/withdraw", h.WithdrawHandler)
		pr.Post("/cashier/accounts/get", h.GetUserAccountsHandler)
	})

	return r
}

