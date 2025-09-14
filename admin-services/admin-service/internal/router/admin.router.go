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
		pr.Post("/partners/create", h.CreatePartner)
		pr.Put("/partners/update", h.UpdatePartner)
		pr.Delete("/partners/delete", h.DeletePartner)

		// ---------------- Partner User Management ----------------
		pr.Post("/partner/users/create", h.CreatePartnerUser)
		pr.Put("/partner/users/update", h.UpdatePartnerUser)
		pr.Delete("/partner/users/delete", h.DeletePartnerUsers)
	})

	// ============================================================
	// URBAC Endpoints
	// ============================================================
	r.Route("/urbac", func(pr chi.Router) {
		// Require role "super_admin"
		pr.Use(auth.Require([]string{"main"}, nil, []string{"super_admin"}))

		// ---------------- USER PERMISSION OVERRIDES ----------------
		pr.Post("/permissions/assign", h.HandleAssignUserPermission)
		pr.Post("/permissions/revoke", h.HandleRevokeUserPermission)
		pr.Get("/permissions/list", h.HandleListUserPermissions)

		// ---------------- MODULES ----------------
		pr.Post("/modules/create", h.HandleCreateModule)
		pr.Put("/modules/update", h.HandleUpdateModule)
		pr.Post("/modules/deactivate", h.HandleDeactivateModule)
		pr.Delete("/modules/delete", h.HandleDeleteModule)
		pr.Get("/modules/list", h.HandleListModules)

		// ---------------- SUBMODULES ----------------
		pr.Post("/submodules/create", h.HandleCreateSubmodule)
		pr.Put("/submodules/update", h.HandleUpdateSubmodule)
		pr.Post("/submodules/deactivate", h.HandleDeactivateSubmodule)
		pr.Delete("/submodules/delete", h.HandleDeleteSubmodule)
		pr.Get("/submodules/list", h.HandleListSubmodules)

		// ---------------- PERMISSION TYPES ----------------
		pr.Post("/permission-types/create", h.HandleCreatePermissionType)
		pr.Put("/permission-types/update", h.HandleUpdatePermissionType)
		pr.Post("/permission-types/deactivate", h.HandleDeactivatePermissionType)
		pr.Get("/permission-types/list", h.HandleListPermissionTypes)

		// ---------------- ROLES ----------------
		pr.Post("/roles/create", h.HandleCreateRole)
		pr.Put("/roles/update", h.HandleUpdateRole)
		pr.Post("/roles/deactivate", h.HandleDeactivateRole)
		pr.Delete("/roles/delete", h.HandleDeleteRole)
		pr.Get("/roles/list", h.HandleListRoles)

		// ---------------- ROLE PERMISSIONS ----------------
		pr.Post("/roles/permissions/assign", h.HandleAssignRolePermission)
		pr.Post("/roles/permissions/revoke", h.HandleRevokeRolePermission)
		pr.Get("/roles/permissions/list", h.HandleListRolePermissions)

		// ---------------- USER ROLES ----------------
		pr.Post("/users/roles/assign", h.HandleAssignUserRole)
		pr.Post("/users/roles/remove", h.HandleRemoveUserRole)
		pr.Get("/users/roles/list", h.HandleListUserRoles)
		pr.Post("/users/roles/upgrade", h.HandleUpgradeUserRole)

		// ---------------- PERMISSION QUERIES ----------------
		pr.Get("/permissions/effective", h.HandleGetEffectiveUserPermissions)
		pr.Post("/permissions/check", h.HandleCheckUserPermission)

		// ---------------- AUDIT LOGS ----------------
		pr.Get("/audit/events", h.HandleListPermissionAuditEvents)
	})

	return r
}
