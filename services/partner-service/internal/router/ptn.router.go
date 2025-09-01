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
        AllowedOrigins:   []string{"http://127.0.0.1:5500", "http://localhost:5500"},
        AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
        AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
        ExposedHeaders:   []string{"Link"},
        AllowCredentials: true,
        MaxAge:           300,
    }))
    r.Use(auth.RateLimit(rdb, 100, time.Minute, 10*time.Minute, "global"))

    uploadDir := "/app/uploads"
    if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
        os.MkdirAll(uploadDir, 0755)
    }

    // ---- Public routes ----
    r.Group(func(pr chi.Router) {
        pr.Get("/health", func(w http.ResponseWriter, r *http.Request) {
            w.WriteHeader(http.StatusOK)
            w.Write([]byte("ok"))
        })
    })

    // ---- Authenticated routes ----
    r.Group(func(pr chi.Router) {
        pr.Use(auth.Require([]string{"main"}, nil)) // ✅ middleware inside group

        pr.Group(func(pr chi.Router) {
            pr.Use(auth.RequireRole([]string{"system_admin"}))
            pr.Delete("/partners/delete/{id}", h.DeletePartner)
        })

        pr.Group(func(pr chi.Router) {
            pr.Use(auth.RequireRole([]string{"system_admin", "partner_admin"}))

            // Serve static files
            pr.Handle("/uploads/*", http.StripPrefix("/uploads/", http.FileServer(http.Dir(uploadDir))))

            // Partner management
            pr.Post("/partners/create", h.CreatePartner)

            // Partner user management
            pr.Post("/partner/users/create", h.CreatePartnerUser)
            pr.Put("/partner/users/update/{id}", h.UpdatePartnerUser)
            pr.Delete("/partner/users/delete/{id}", h.DeletePartnerUser) // fixed typo: missing slash
        })
    })

    return r
}

