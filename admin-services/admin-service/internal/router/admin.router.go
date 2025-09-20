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
		pr.Use(auth.Require([]string{"main"}, nil, []string{"super_admin"}))

		// ---------------- Partner Management ----------------
		// ---------------- Partner Management ----------------
		pr.Route("/partners", func(p chi.Router) {
			p.Post("/create", h.CreatePartner)
			p.Put("/update", h.UpdatePartner)
			p.Delete("/delete", h.DeletePartner)

			// ---------------- Partner User Management ----------------
			p.Route("/users", func(u chi.Router) {
				u.Post("/create", h.CreatePartnerUser)
				u.Put("/update", h.UpdatePartnerUser)
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
		pr.Route("/accounting", func(acc chi.Router) {
			acc.Post("/accounts", h.CreateAccounts)
			acc.Post("/accounts/get", h.GetUserAccounts)

			acc.Post("/transactions", h.PostTransaction)

			acc.Post("/statements/account", h.GetAccountStatement)
			acc.Post("/statements/owner", h.GetOwnerStatement)

			acc.Post("/journal/postings", h.GetJournalPostings)

			acc.Post("/reports/daily", h.GenerateDailyReport)
		})
	})

	return r
}

