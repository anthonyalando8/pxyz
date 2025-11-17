// router/router.go - UPDATED VERSION

package router

import (
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/redis/go-redis/v9"

	"partner-service/internal/handler"
	"x/shared/auth/middleware"
)

func SetupRoutes(
	r chi.Router,
	h *handler.PartnerHandler,
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
	r.Use(auth.RateLimit(rdb, 100, time.Minute, 10*time.Minute, "global"))

	uploadDir := "/app/uploads"
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		os.MkdirAll(uploadDir, 0755)
	}

	// ---- Mount all routes under /partner/svc ----
	r.Route("/partner/svc", func(pr chi.Router) {

		// ---- Public routes ----
		pr.Group(func(pub chi.Router) {
			pub.Get("/health", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ok"))
			})
		})

		// ---- Admin-only routes ----
		pr.Group(func(admin chi.Router) {
			admin.Use(auth.Require([]string{"main"}, nil, []string{"partner_admin"}))

			admin.Handle("/uploads/*", http.StripPrefix("/partner/svc/uploads/", http.FileServer(http.Dir(uploadDir))))
			
			// User management (admin only)
			admin.Post("/users/create", h.CreatePartnerUser)
			admin.Delete("/users/delete/{id}", h.DeletePartnerUser)
			admin.Put("/users/{id}/status", h.UpdatePartnerUserStatus)
			admin.Put("/users/{id}/role", h.UpdatePartnerUserRole)
			admin.Post("/users/bulk/status", h.BulkUpdateUserStatus)
			
			// User listing and search (admin only)
			admin.Get("/users/stats", h.GetPartnerUserStats)
			admin.Post("/users/list", h.ListPartnerUsers)
			admin.Post("/users/search", h.SearchPartnerUsers)
			admin.Post("/users/email", h.GetPartnerUserByEmail)

			// ---- API Credentials Management (admin only) ----
			admin.Route("/api", func(api chi.Router) {
				api.Post("/credentials/generate", h.GenerateAPICredentials)
				api.Delete("/credentials/revoke", h.RevokeAPICredentials)
				api.Post("/credentials/rotate", h.RotateAPISecret)
				api.Get("/settings", h.GetAPISettings)
				api.Put("/settings", h.UpdateAPISettings)
			})

			// ---- Webhook Management (admin only) ----
			admin.Route("/webhooks", func(wh chi.Router) {
				wh.Put("/config", h.UpdateWebhookConfig)
				wh.Post("/test", h.TestWebhook)
				wh.Get("/logs", h.ListWebhookLogs)
				wh.Post("/{id}/retry", h.RetryFailedWebhook)
			})

			// ---- API Logs & Analytics (admin only) ----
			admin.Get("/api/logs", h.GetAPILogs)
			admin.Post("/api/usage", h.GetAPIUsageStats)
		})

		// ---- Admin + User routes ----
		pr.Group(func(shared chi.Router) {
			shared.Use(auth.Require([]string{"main"}, nil, []string{"partner_admin", "partner_user"}))

			// ---- Transaction Management (admin + user) ----
			shared.Route("/transactions", func(txn chi.Router) {
				txn.Post("/deposit", h.InitiateDeposit)
				txn.Get("/{ref}", h.GetTransactionStatus)
				txn.Get("/", h.ListTransactions)
				txn.Post("/search", h.GetTransactionsByDateRange)
			})

			// Accounting routes
			shared.Route("/accounting", func(a chi.Router) {
				a.Get("/accounts/get", h.GetUserAccounts)
				a.Post("/account/statement", h.GetAccountStatement)
				a.Post("/owner/statement", h.GetOwnerStatement)
			})
		})
	})

	return r
}