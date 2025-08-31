package router

import (
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/redis/go-redis/v9"

	"auth-service/internal/handler"
	"x/shared/auth/middleware"
)

func SetupRoutes(
	r chi.Router,
	h *handler.AuthHandler,
	auth *middleware.MiddlewareWithClient,
	wsHandler *handler.WSHandler,
	rdb *redis.Client,
) chi.Router {
	// ---- Global Middleware ----
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://127.0.0.1:5500", "http://localhost:5500", "https://4bcbc3ea8466.ngrok-free.app"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Use(auth.RateLimit(rdb, 100, time.Minute, 10*time.Minute, "global"))

	uploadDir := "/app/uploads"

	// Ensure directory exists
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		os.MkdirAll(uploadDir, 0755)
	}
	
	// Login attempts (stricter)
	r.Group(func(r chi.Router) {
		r.Use(auth.RateLimit(rdb, 5, 30*time.Second, 30*time.Second, "auth"))
		// ============================================================
		// Public Endpoints (no auth required)
		// ============================================================
		r.Post("/auth/exists", h.HandleUserExists)
		r.Post("/auth/register/init", h.HandleInitSignup)
		r.Post("/auth/login", h.HandleLogin)
		r.Post("/auth/google", h.GoogleAuthHandler)
		r.Post("/auth/telegram", h.TelegramLogin)
		r.Post("/auth/apple", h.AppleAuthHandler)
		r.Post("/auth/password/forgot", h.HandleForgotPassword) // reset after forgot-password
	})
	
	r.Group(func(pr chi.Router) {
		pr.Use(auth.Middleware) //must have valid session (any)
		pr.Post("/auth/register/otp/request", h.HandleRequestOTP)
	})


	// ============================================================
	// Registration & OTP flows (require temp/main session with purpose)
	// ============================================================
	r.Group(func(pr chi.Router) {
		// Password setup flows
		pr.Use(auth.Require([]string{"main", "temp"}, []string{"register"}))
		pr.Post("/auth/password/set", h.HandleSetPassword)     // first-time set during signup
	})
	
	r.Group(func(pr chi.Router) {
		// Password setup flows
		pr.Use(auth.Require([]string{"temp"}, []string{"password_reset"}))
		pr.Post("/auth/password/reset", h.HandleResetPassword) // reset after forgot-password
	})

	r.Group(func(pr chi.Router) {
		// OTP request/verify (register/email change flows)
		pr.Use(auth.Require([]string{"main", "temp"}, []string{"register", "email_change", "incomplete_profile","general", "verify-otp"}))
		pr.Post("/auth/register/otp/request", h.HandleRequestOTP)
		pr.Post("/auth/register/otp/verify", h.HandleVerifyOTP)
	})

	// ============================================================
	// Email change (OTP-protected)
	// ============================================================
	r.Group(func(pr chi.Router) {
		pr.Use(auth.Require([]string{"main", "temp"}, []string{"email_change"}))
		pr.Patch("/auth/email", h.HandleChangeEmail)
	})

	// ============================================================
	// Authenticated User Endpoints (require main session)
	// ============================================================
	r.Group(func(pr chi.Router) {
		pr.Use(auth.Require([]string{"main"}, nil))

		// Serve /uploads/... -> files from ./uploads/
		pr.Handle("/uploads/*", http.StripPrefix("/uploads/", http.FileServer(http.Dir(uploadDir))))
		// WebSocket
		pr.Get("/auth/ws", wsHandler.HandleWS)

		// 2FA
		pr.Get("/auth/2fa/init", h.HandleInitiate2FA)
		pr.Post("/auth/2fa/enable", h.HandleEnable2FA)
		pr.Post("/auth/2fa/disable", h.HandleDisable2FA)
		pr.Post("/auth/2fa/verify", h.HandleVerify2FA)
		pr.Get("/auth/2fa/status", h.Handle2FAStatus)

		// Profile
		pr.Get("/auth/profile", h.HandleProfile) // get full profile
		pr.Post("/auth/profile/update", h.HandleUpdateProfile) // partial updates
		pr.Patch("/auth/name", h.HandleUpdateName)
		pr.Patch("/auth/phone", h.HandlePhoneChange)           // via 2FA
		pr.Get("/auth/email/request-change", h.HandleRequestEmailChange) // via 2FA
		pr.Post("/auth/profile/picture", h.UploadProfilePicture)

		// Password management
		pr.Patch("/auth/password/change", h.HandleChangePassword) // change existing password
		// pr.Post("/auth/password/convert", h.HandleConvertPassword) // social → hybrid

		// Sessions
		pr.Get("/auth/sessions", h.ListSessionsHandler(auth.Client))
		pr.Delete("/auth/logout", h.LogoutHandler(auth.Client))
		pr.Delete("/auth/sessions", h.LogoutAllHandler(auth.Client, rdb))
		pr.Delete("/auth/sessions/{id}", h.DeleteSessionByIDHandler(auth.Client))
	})

	return r
}
