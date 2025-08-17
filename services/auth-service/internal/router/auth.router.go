package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/redis/go-redis/v9"

	"auth-service/internal/handler"
	"x/shared/auth/middleware"
)

func SetupRoutes(r chi.Router, h *handler.AuthHandler, auth *middleware.MiddlewareWithClient, wsHandler *handler.WSHandler, rdb *redis.Client) chi.Router {
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://127.0.0.1:5500", "http://localhost:5500"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// ---- Public endpoints ----
	r.Post("/auth/exists", h.HandleUserExists)
	r.Post("/auth/register/init", h.HandleInitSignup)
	r.Post("/auth/login", h.HandleLogin)

	// ---- Protected OTP verify (any session type, but purpose = "register") ----
	r.Group(func(pr chi.Router) {
		pr.Use(auth.Require([]string{"main", "temp"}, []string{"register"}))
		pr.Post("/auth/register/otp/request", h.HandleRequestOTP)
		pr.Post("/auth/register/otp/verify", h.HandleVerifyOTP)
		pr.Post("/auth/password/set", h.HandleSetPassword)            // first-time set during signup
		pr.Post("/auth/password/reset", h.HandleResetPassword)        // reset after forgot-password flow

		pr.Patch("/auth/email", h.HandleChangeEmail) //INITIATED VIA 2FA

	})

	// ---- Authenticated user routes ----
	r.Group(func(pr chi.Router) {
		pr.Use(auth.Require([]string{"main"}, nil))

		// WebSocket endpoint
		pr.Get("/auth/ws", wsHandler.HandleWS)

		// Profile management
		pr.Get("/auth/email/request-update", h.HandleChangeEmail) //INITIATED VIA 2FA
		pr.Patch("/auth/name", h.HandleUpdateName)

		// Password flows
		pr.Patch("/auth/password", h.HandleChangePassword)            // change existing password (requires old + new)
		//pr.Post("/auth/password/convert", h.HandleConvertPassword)    // set when converting social → hybrid

		// Sessions
		pr.Delete("/auth/logout", h.LogoutHandler(auth.Client))
		pr.Delete("/auth/sessions", h.LogoutAllHandler(auth.Client, rdb))
		pr.Delete("/auth/sessions/{id}", h.DeleteSessionByIDHandler(auth.Client))

		pr.Get("/auth/sessions", h.ListSessionsHandler(auth.Client))
		pr.Get("/auth/profile", h.HandleProfile)
	})

	return r
}

