package router

import (
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/redis/go-redis/v9"

	"admin-service/internal/handler"
	"x/shared/auth/middleware"
)

func SetupRoutes(
	r chi.Router,
	h *handler.AdminHandler,
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
	// Authenticated Endpoints (Admin actions)
	// ============================================================
	r.Route("/admin/svc", func(pr chi.Router) {
		// Require role "admin" (adjust if multiple roles allowed)
		pr.Use(auth.Require([]string{"main"}, nil, []string{"super_admin"}))

		// ---------------- Partner Management ----------------
		pr.Post("/partners", h.CreatePartner)
		pr.Put("/partners", h.UpdatePartner)
		pr.Delete("/partners", h.DeletePartner)

		// ---------------- Partner User Management ----------------
		pr.Post("/partners/users", h.CreatePartnerUser)
		pr.Put("/partners/users", h.UpdatePartnerUser)
		pr.Delete("/partners/users", h.DeletePartnerUsers)
	})

	return r
}
