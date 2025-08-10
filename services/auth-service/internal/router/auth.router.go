package router

import (
	"github.com/go-chi/chi/v5"

	"auth-service/internal/handler"
	"x/shared/auth/middleware"
)

func SetupRoutes(r chi.Router, h *handler.AuthHandler, auth *middleware.MiddlewareWithClient) chi.Router {
	r.Post("/auth/exists", h.HandleUserExists)

	r.Post("/auth/register", h.HandleRegister)
	r.Post("/auth/login", h.HandleLogin)

	r.Group(func(pr chi.Router) {
		pr.Use(auth.Middleware)
		
		pr.Patch("/auth/email", h.HandleChangeEmail)        // PATCH for partial updates
		pr.Patch("/auth/password", h.HandleChangePassword)  // PATCH for partial updates
		pr.Patch("/auth/name", h.HandleUpdateName)          // Update first and last name

		pr.Delete("/auth/logout", h.LogoutHandler(auth.Client))
		pr.Delete("/auth/sessions", h.LogoutAllHandler(auth.Client))
		pr.Delete("/auth/sessions/{id}", h.DeleteSessionByIDHandler(auth.Client))

		pr.Get("/auth/sessions", h.ListSessionsHandler(auth.Client))

		pr.Get("/auth/profile", h.HandleProfile) // Get user profile
	})


	return r
}
