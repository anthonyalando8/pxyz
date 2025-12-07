// router/router.go - UPDATED VERSION

package router

import (
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/redis/go-redis/v9"
	ptnmiddleware "partner-service/pkg/auth"

	"partner-service/internal/handler"
	"x/shared/auth/middleware"
)

func SetupRoutes(
	r chi.Router,
	h *handler. PartnerHandler,
	auth *middleware. MiddlewareWithClient,
	ptnAuth *ptnmiddleware. APIKeyAuthMiddleware,
	rdb *redis. Client,
) chi.Router {
	// ---- Global Middleware ----
	r. Use(cors.Handler(cors. Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-API-Key", "X-API-Secret", "ngrok-skip-browser-warning"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	r.Use(auth.RateLimit(rdb, 100, time.Minute, 10*time.Minute, "global"))

	uploadDir := "/app/uploads"
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		os.MkdirAll(uploadDir, 0755)
	}

	// ============================================================================
	// API KEY AUTHENTICATED ROUTES (External Partner API)
	// ============================================================================
	r.Route("/partner/api/", func(api chi.Router) {
		// Apply API key authentication middleware
		api.Use(ptnAuth.RequireAPIKey())
	
		// Credit user wallet (external API call)
		api.Post("/transactions/credit", h.CreditUser)
		
		// Query transaction status by partner reference
		api.Get("/transactions/{ref}", h.GetTransactionByRef)
		
		// List partner transactions
		api.Get("/transactions", h.ListPartnerTransactions)
	})

	// ============================================================================
	// INTERNAL AUTHENTICATED ROUTES (Partner Portal)
	// ============================================================================
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
			admin. Use(auth.Require([]string{"main"}, nil, []string{"partner_admin"}))

			admin.Handle("/uploads/*", http.StripPrefix("/partner/svc/uploads/", http. FileServer(http.Dir(uploadDir))))
			
			// ---------------- User Management (admin only) ----------------
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

			// ---------------- API Credentials Management (admin only) ----------------
			admin.Route("/api", func(api chi. Router) {
				api.Post("/credentials/generate", h.GenerateAPICredentials)
				api.Delete("/credentials/revoke", h.RevokeAPICredentials)
				api.Post("/credentials/rotate", h.RotateAPISecret)
				api.Get("/settings", h.GetAPISettings)
				api.Put("/settings", h.UpdateAPISettings)
				api.Get("/logs", h.GetAPILogs)
				api.Post("/usage", h.GetAPIUsageStats)
			})

			// ---------------- Webhook Management (admin only) ----------------
			admin.Route("/webhooks", func(wh chi.Router) {
				wh.Put("/config", h.UpdateWebhookConfig)
				wh.Post("/test", h. TestWebhook)
				wh.Get("/logs", h. ListWebhookLogs)
				wh.Post("/{id}/retry", h.RetryFailedWebhook)
			})
		})

		// ---- Admin + User routes (Shared Access) ----
		pr.Group(func(shared chi.Router) {
			shared.Use(auth.Require([]string{"main"}, nil, []string{"partner_admin", "partner_user"}))

			// ---------------- Transaction Management ----------------
			shared.Route("/transactions", func(txn chi.Router) {
				// Partner transaction operations
				txn.Post("/deposit", h.InitiateDeposit)
				txn.Get("/{ref}", h.GetTransactionByRef)
				txn.Get("/", h.ListPartnerTransactions)
				txn.Post("/search", h.GetTransactionsByDateRange)
			})

			// ---------------- Accounting Routes ----------------
			shared.Route("/accounting", func(acc chi.Router) {
				// Account Management
				acc.Get("/accounts", h.GetUserAccounts)                    // List partner accounts
				acc.Get("/accounts/{number}/balance", h.GetAccountBalance) // Get account balance
				acc.Get("/summary", h.GetOwnerSummary)                     // Get consolidated summary

				// Statements
				acc.Route("/statements", func(stmt chi.Router) {
					stmt.Post("/account", h.GetAccountStatement) // Get account statement
					stmt. Post("/owner", h.GetOwnerStatement)     // Get owner statement (all accounts)
				})

				// Transaction Queries
				acc.Route("/transactions", func(tx chi.Router) {
					tx.Get("/{receipt}", h.GetTransactionByReceipt)    // Get transaction by receipt code
					tx.Get("/", h.ListPartnerTransactions)             // List all partner transactions
					tx.Get("/ref/{ref}", h.GetTransactionByRef)        // Get by partner reference
				})

				// Ledger Queries
				acc.Route("/ledgers", func(ledg chi.Router) {
					ledg.Get("/account/{number}", h.GetAccountLedgers) // Get ledgers for account
				})
			})
		})
	})

	return r
}