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
		// Require role "super_admin"
		pr.Use(auth.Require([]string{"main"}, nil, nil))

		// ---------------- Partner Management ----------------
		pr.Route("/partners", func(p chi.Router) {
			p.Post("/create", h.CreatePartner)
			p.Put("/update", h.UpdatePartner)
			p.Delete("/delete", h.DeletePartner)

			// ---------------- Partner User Management ----------------
			p.Route("/users", func(u chi.Router) {
				u.Post("/create", h.CreatePartnerUser)
				u.Delete("/delete", h.DeletePartnerUsers)
			})
		})

		// ============================================================
		// URBAC Endpoints (under /admin/svc/urbac)
		// ============================================================
		pr.Route("/urbac", func(up chi.Router) {
			// ---------------- USER PERMISSION OVERRIDES ----------------
			up.Post("/permissions/assign", h.HandleAssignUserPermission)
			up.Post("/permissions/revoke", h.HandleRevokeUserPermission)
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
			up.Put("/permission-types/update", h.HandleUpdatePermissionType)
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
			up.Post("/roles/permissions/revoke", h.HandleRevokeRolePermission)
			up.Get("/roles/permissions/list", h.HandleListRolePermissions)

			// ---------------- USER ROLES ----------------
			up.Post("/users/roles/assign", h.HandleAssignUserRole)
			up.Post("/users/roles/remove", h.HandleRemoveUserRole)
			up.Get("/users/roles/list", h.HandleListUserRoles)
			up.Post("/users/roles/upgrade", h.HandleUpgradeUserRole)

			// ---------------- PERMISSION QUERIES ----------------
			up.Get("/permissions/effective", h.HandleGetEffectiveUserPermissions)
			up.Post("/permissions/check", h.HandleCheckUserPermission)

			// ---------------- AUDIT LOGS ----------------
			up.Get("/audit/events", h.HandleListPermissionAuditEvents)
		})

		// ============================================================
		// ACCOUNTING Endpoints (under /admin/svc/accounting)
		// ============================================================
		pr.Route("/accounting", func(acc chi.Router) {
			// ---------------- Account Management ----------------
			acc.Post("/accounts", h.CreateAccounts)
			acc.Post("/accounts/user", h.GetUserAccounts)
			acc.Get("/accounts/{number}/balance", h.GetAccountBalance)
			acc.Put("/accounts/{id}", h.UpdateAccount)

			// ---------------- Transaction Operations ----------------
			acc.Route("/transactions", func(tx chi.Router) {
				tx.Post("/credit", h.CreditAccount)
				tx.Post("/debit", h.DebitAccount)
				tx.Post("/transfer", h.TransferFunds)
				tx.Post("/convert", h.ConvertAndTransfer)
				tx.Post("/trade/win", h.ProcessTradeWin)
				tx.Post("/trade/loss", h.ProcessTradeLoss)
				tx.Post("/commission", h.ProcessAgentCommission)
				tx.Get("/{receipt}", h.GetTransactionByReceipt)
			})

			// ---------------- Approval Management ----------------
			acc.Route("/approvals", func(apr chi.Router) {
				apr.Get("/pending", h.GetPendingApprovals)
				apr.Post("/{id}/approve", h.ApproveTransaction)
				apr.Get("/history", h.GetApprovalHistory)
			})

			// ---------------- Statements ----------------
			acc.Route("/statements", func(stmt chi.Router) {
				stmt.Post("/account", h.GetAccountStatement)
				stmt.Post("/owner", h.GetOwnerStatement)
				stmt.Get("/summary/{owner_type}/{owner_id}", h.GetOwnerSummary)
			})

			// ---------------- Ledgers ----------------
			acc.Route("/ledgers", func(ledg chi.Router) {
				ledg.Get("/account/{number}", h.GetAccountLedgers)
				ledg.Get("/journal/{id}", h.GetJournalLedgers)
			})

			// ---------------- Reports & Analytics ----------------
			acc.Route("/reports", func(rpt chi.Router) {
				rpt.Get("/daily", h.GenerateDailyReport)
				rpt.Get("/transaction-summary", h.GetTransactionSummary)
				rpt.Get("/system-holdings", h.GetSystemHoldings)
			})

			// ---------------- Fee Management ----------------
			acc.Route("/fees", func(fee chi.Router) {
				fee.Get("/calculate", h.CalculateFee)
				fee.Get("/receipt/{receipt}", h.GetFeesByReceipt)
				fee.Get("/commission/{agent_id}", h.GetAgentCommissionSummary)
			})

			// ---------------- Agent Management ----------------
			acc.Route("/agents", func(agt chi.Router) {
				agt.Post("/", h.CreateAgent)
				agt.Get("/", h.ListAgents)
				agt.Get("/{agent_id}", h.GetAgentByID)
				agt.Put("/{agent_id}", h.UpdateAgent)
				agt.Delete("/{agent_id}", h.DeleteAgent)
				agt.Get("/stats", h.GetAgentStats)
				agt.Get("/by-countries", h.GetAgentsByCountries)
				agt.Get("/user/{user_id}", h.GetAgentByUserID)
				agt.Get("/{agent_id}/commissions", h.ListCommissionsForAgent)
			})
		})

		// ============================================================
		//  CRYPTO Endpoints (under /admin/svc/crypto)
		// ============================================================
		pr.Route("/crypto", func(crypto chi.Router) {
			// ---------------- User Wallet Management ----------------
			crypto.Route("/wallets", func(w chi.Router) {
				// Batch operations
				w.Post("/batch", h.CreateWallets)                  // Create multiple wallets
				w.Post("/initialize", h.InitializeUserWallets)     // Initialize all wallets for user
				
				// Query operations
				w.Get("/user", h.GetUserWallets)                   // Get user wallets (query param: user_id)
				w.Get("/address", h.GetWalletByAddress)            // Get wallet by address (query param: address)
				
				// Balance operations
				w.Post("/refresh", h.RefreshBalance)               // Force balance refresh (query params: wallet_id, user_id)
			})

			// ---------------- System Wallet Management (Hot Wallets) ----------------
			crypto.Route("/system", func(sys chi.Router) {
				// System wallet queries
				sys.Get("/wallets", h.GetSystemWallets)            // Get all system hot wallets
				sys.Get("/balance", h.GetSystemBalance)            // Get system balance (query params: chain, asset, force_refresh)
				sys.Get("/wallet", h.GetSystemWalletByAsset)       // Get specific system wallet (query params: chain, asset)
			})

			// ---------------- Transaction Management ----------------
			crypto.Route("/transactions", func(tx chi.Router) {
				tx.Get("/", h.GetTransaction)                    // Get by transaction_id query param
				tx.Get("/user", h.GetUserTransactions)           // Get user transactions
				tx.Get("/status", h.GetTransactionStatus)        // Get transaction status
			})

			// ---------------- Approval Management ----------------
			crypto.Route("/approvals", func(apr chi.Router) {
				apr.Get("/pending", h.GetPendingWithdrawals)    // Get pending approvals
				apr.Get("/", h.GetWithdrawalApproval)            // Get approval by approval_id query param
				apr.Post("/approve", h.ApproveWithdrawal)        // Approve withdrawal
				apr.Post("/reject", h.RejectWithdrawal)          // Reject withdrawal
			})

			// ---------------- Sweep Operations (System Maintenance) ----------------
			crypto.Route("/sweep", func(swp chi.Router) {
				swp.Post("/user", h.SweepUserWallet)             // Sweep single user
				swp.Post("/all", h.SweepAllUsers)                // Sweep all users
			})

			// ---------------- Fee Estimation ----------------
			crypto.Route("/fees", func(fee chi.Router) {
				fee.Get("/estimate", h.EstimateNetworkFee)       // Estimate network fee
			})
			
			// ---------------- Deposit Management ----------------
			crypto.Route("/deposits", func(dep chi.Router) {
				// Query operations
				dep.Get("/", h.GetDeposit)                      // Get by deposit_id query param
				dep.Get("/user", h.GetUserDeposits)             // Get user deposits
				dep.Get("/all", h.GetAllDeposits)               // Get all deposits (admin overview)
				dep.Get("/pending", h.GetPendingDeposits)       // Get pending deposits
				dep.Get("/recent", h.GetRecentDeposits)         // Get recent deposits
				dep.Get("/tx", h.GetDepositByTxHash)            // Search by tx_hash query param
				
				// Statistics
				dep.Get("/stats", h.GetDepositStats)            // Get deposit statistics
				
				// Admin interventions (TODO - implement when needed)
				dep.Post("/retry", h.RetryDepositProcessing)    // Retry stuck deposit
				dep.Post("/fail", h.MarkDepositAsFailed)        // Mark deposit as failed
			})
			// ---------------- Monitoring & Reports ----------------
			crypto.Route("/monitoring", func(mon chi.Router) {
				// TODO: Add monitoring endpoints
				// mon.Get("/health", h.GetCryptoSystemHealth)
				// mon.Get("/balances/summary", h.GetBalancesSummary)
				// mon.Get("/deposits/recent", h.GetRecentDeposits)
				// mon.Get("/withdrawals/recent", h.GetRecentWithdrawals)
			})
		})
	})

	return r
}