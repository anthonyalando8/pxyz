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
	auth *middleware. MiddlewareWithClient,
	rdb *redis.Client,
) chi.Router {
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
	r.Use(auth.RateLimit(rdb, 100, time.Minute, 10*time. Minute, "global"))

	// ============================================================
	// Authenticated Endpoints (Admin actions)
	// ============================================================
	r.Route("/admin/svc", func(pr chi.Router) {
		// Require role "super_admin"
		pr.Use(auth. Require([]string{"main"}, nil, nil))

		// ---------------- Partner Management ----------------
		pr.Route("/partners", func(p chi. Router) {
			p.Post("/create", h.CreatePartner)
			p.Put("/update", h.UpdatePartner)
			p.Delete("/delete", h.DeletePartner)

			// ---------------- Partner User Management ----------------
			p.Route("/users", func(u chi.Router) {
				u.Post("/create", h.CreatePartnerUser)
				u.Delete("/delete", h. DeletePartnerUsers)
			})
		})

		// ============================================================
		// URBAC Endpoints (under /admin/svc/urbac)
		// ============================================================
		pr.Route("/urbac", func(up chi. Router) {
			// ---------------- USER PERMISSION OVERRIDES ----------------
			up.Post("/permissions/assign", h.HandleAssignUserPermission)
			up.Post("/permissions/revoke", h. HandleRevokeUserPermission)
			up.Get("/permissions/list", h.HandleListUserPermissions)

			// ---------------- MODULES ----------------
			up.Post("/modules/create", h.HandleCreateModule)
			up.Put("/modules/update", h.HandleUpdateModule)
			up.Post("/modules/deactivate", h.HandleDeactivateModule)
			up.Delete("/modules/delete", h.HandleDeleteModule)
			up.Get("/modules/list", h.HandleListModules)

			// ---------------- SUBMODULES ----------------
			up.Post("/submodules/create", h.HandleCreateSubmodule)
			up.Put("/submodules/update", h.HandleUpdateSubmodule)
			up.Post("/submodules/deactivate", h.HandleDeactivateSubmodule)
			up.Delete("/submodules/delete", h.HandleDeleteSubmodule)
			up.Get("/submodules/list", h.HandleListSubmodules)

			// ---------------- PERMISSION TYPES ----------------
			up.Post("/permission-types/create", h.HandleCreatePermissionType)
			up.Put("/permission-types/update", h. HandleUpdatePermissionType)
			up.Post("/permission-types/deactivate", h.HandleDeactivatePermissionType)
			up.Get("/permission-types/list", h.HandleListPermissionTypes)

			// ---------------- ROLES ----------------
			up.Post("/roles/create", h.HandleCreateRole)
			up.Put("/roles/update", h.HandleUpdateRole)
			up.Post("/roles/deactivate", h.HandleDeactivateRole)
			up.Delete("/roles/delete", h.HandleDeleteRole)
			up.Get("/roles/list", h.HandleListRoles)

			// ---------------- ROLE PERMISSIONS ----------------
			up.Post("/roles/permissions/assign", h.HandleAssignRolePermission)
			up.Post("/roles/permissions/revoke", h. HandleRevokeRolePermission)
			up.Get("/roles/permissions/list", h.HandleListRolePermissions)

			// ---------------- USER ROLES ----------------
			up.Post("/users/roles/assign", h.HandleAssignUserRole)
			up. Post("/users/roles/remove", h.HandleRemoveUserRole)
			up.Get("/users/roles/list", h.HandleListUserRoles)
			up. Post("/users/roles/upgrade", h.HandleUpgradeUserRole)

			// ---------------- PERMISSION QUERIES ----------------
			up. Get("/permissions/effective", h. HandleGetEffectiveUserPermissions)
			up.Post("/permissions/check", h.HandleCheckUserPermission)

			// ---------------- AUDIT LOGS ----------------
			up.Get("/audit/events", h.HandleListPermissionAuditEvents)
		})

		// ============================================================
		// ACCOUNTING Endpoints (under /admin/svc/accounting)
		// ============================================================
		pr.Route("/accounting", func(acc chi.Router) {
			// ---------------- Account Management ----------------
			acc.Post("/accounts", h.CreateAccounts)                    // Create accounts (batch)
			acc.Post("/accounts/user", h.GetUserAccounts)              // Get user accounts by owner
			acc.Get("/accounts/{number}/balance", h.GetAccountBalance) // Get account balance
			acc.Put("/accounts/{id}", h.UpdateAccount)                 // Update account settings

			// ---------------- Transaction Operations ----------------
			acc.Route("/transactions", func(tx chi.Router) {
				// Credit/Debit operations (approval-based for regular admins)
				tx.Post("/credit", h.CreditAccount) // Credit account
				tx.Post("/debit", h.DebitAccount)   // Debit account

				// Transfer operations
				tx.Post("/transfer", h.TransferFunds)   // Transfer between accounts
				tx.Post("/convert", h.ConvertAndTransfer) // Convert and transfer

				// Trade operations
				tx.Post("/trade/win", h.ProcessTradeWin)   // Process trade win
				tx.Post("/trade/loss", h.ProcessTradeLoss) // Process trade loss

				// Commission operations
				tx.Post("/commission", h.ProcessAgentCommission) // Process agent commission

				// Transaction queries
				tx.Get("/{receipt}", h.GetTransactionByReceipt) // Get transaction by receipt code
			})

			// ---------------- Approval Management ----------------
			acc.Route("/approvals", func(apr chi.Router) {
				apr.Get("/pending", h.GetPendingApprovals)          // Get pending approvals (super admin)
				apr.Post("/{id}/approve", h.ApproveTransaction)     // Approve/reject transaction (super admin)
				apr.Get("/history", h.GetApprovalHistory)           // Get approval history
			})

			// ---------------- Statements ----------------
			acc.Route("/statements", func(stmt chi.Router) {
				stmt.Post("/account", h.GetAccountStatement) // Get account statement
				stmt. Post("/owner", h.GetOwnerStatement)     // Get owner statement
				stmt.Get("/summary/{owner_type}/{owner_id}", h.GetOwnerSummary) // Get owner summary
			})

			// ---------------- Ledgers ----------------
			acc.Route("/ledgers", func(ledg chi.Router) {
				ledg.Get("/account/{number}", h.GetAccountLedgers) // Get ledgers for account
				ledg.Get("/journal/{id}", h.GetJournalLedgers)     // Get ledgers for journal
			})

			// ---------------- Reports & Analytics ----------------
			acc.Route("/reports", func(rpt chi.Router) {
				rpt.Get("/daily", h.GenerateDailyReport)             // Generate daily report
				rpt. Get("/transaction-summary", h.GetTransactionSummary) // Get transaction summary
				rpt.Get("/system-holdings", h.GetSystemHoldings)     // Get system holdings
			})

			// ---------------- Fee Management ----------------
			acc.Route("/fees", func(fee chi. Router) {
				fee.Get("/calculate", h. CalculateFee)                   // Calculate fee (preview)
				fee.Get("/receipt/{receipt}", h.GetFeesByReceipt)       // Get fees by receipt
				fee.Get("/commission/{agent_id}", h.GetAgentCommissionSummary) // Get agent commission summary
			})
		})
	})

	return r
}