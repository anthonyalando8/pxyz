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
		AllowedOrigins:   []string{"*"}, // allow all origins
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "ngrok-skip-browser-warning"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false, // must be false when using "*"
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

		// ---- Authenticated routes ----
		pr.Group(func(priv chi.Router) {
			// --- All routes require partner_admin by default ---
			priv.Use(auth.Require([]string{"main"}, nil, []string{"partner_admin"}))

			priv.Handle("/uploads/*", http.StripPrefix("/partner/svc/uploads/", http.FileServer(http.Dir(uploadDir))))
			priv.Delete("/users/delete/{id}", h.DeletePartnerUser)

			// Partner user management
			priv.Post("/users/create", h.CreatePartnerUser)

			// --- Update can be accessed by both partner_admin and partner_user ---
			priv.With(auth.Require([]string{"main"}, nil, []string{"partner_admin", "partner_user"})).
				Put("/users/update/{id}", h.UpdatePartnerUser)
		})

		// ---- Accounting routes (admin or user) ----
		pr.Group(func(acc chi.Router) {
			acc.Use(auth.Require([]string{"main"}, nil, []string{"partner_admin", "partner_user"}))

			acc.Route("/accounting", func(a chi.Router) {
				a.Get("/accounts/get", h.GetUserAccounts)
				a.Post("/account/statement", h.GetAccountStatement)
				a.Post("/owner/statement", h.GetOwnerStatement)
			})
		})
	})


	return r
}



