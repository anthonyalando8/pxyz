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

	// ============================================================
	// Public Endpoints (No Auth Required)
	// ============================================================
	r.Group(func(r chi.Router) {
		r.Use(auth.RateLimit(rdb, 5, 30*time.Second, 30*time.Second, "auth"))
		r.Post("/auth/exists", h.HandleUserExists)
		r.Post("/auth/register/init", h.HandleInitSignup)
		r.Post("/auth/login", h.HandleLogin)
		r.Post("/auth/google", h.GoogleAuthHandler)
		r.Post("/auth/telegram", h.TelegramLogin)
		r.Post("/auth/apple", h.AppleAuthHandler)
		r.Post("/auth/password/forgot", h.HandleForgotPassword)
		// Serve uploads
		r.Handle("/auth/uploads/*", http.StripPrefix("/auth/uploads/", http.FileServer(http.Dir(uploadDir))))
	})

	// ============================================================
	// Registration & Password Flows (require specific session/purpose)
	// ============================================================
	r.Group(func(r chi.Router) {
		r.Use(auth.Require([]string{"main", "temp"}, []string{"register"}, nil))
		r.Post("/auth/password/set", h.HandleSetPassword)
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.Require([]string{"temp"}, []string{"password_reset"}, nil))
		r.Post("/auth/password/reset", h.HandleResetPassword)
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.Require([]string{"main", "temp"}, []string{"register", "email_change", "incomplete_profile", "general", "verify-otp", "phone_change"}, nil))
		r.Post("/auth/register/otp/request", h.HandleRequestOTP)
		r.Post("/auth/register/otp/verify", h.HandleVerifyOTP)
	})

	// Email & Phone Change (OTP-protected)
	r.Group(func(r chi.Router) {
		r.Use(auth.Require([]string{"main", "temp"}, []string{"email_change"}, nil))
		r.Patch("/auth/email", h.HandleChangeEmail)
	})
	r.Group(func(r chi.Router) {
		r.Use(auth.Require([]string{"main", "temp"}, []string{"phone_change"}, nil))
		r.Patch("/auth/phone/update", h.HandlePhoneChange)
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.Require([]string{"main", "temp"}, []string{"general", "register","incomplete_profile"}, nil))
		r.Post("/auth/profile/nationality", h.HandleUpdateNationality)
	})

	// ============================================================
	// Authenticated User Endpoints (main session required)
	// ============================================================
	r.Group(func(pr chi.Router) {
		pr.Use(auth.Require([]string{"main"}, nil, nil))


		// WebSocket
		pr.Get("/auth/ws", wsHandler.HandleWS)

		// 2FA
		pr.Get("/auth/2fa/init", h.HandleInitiate2FA)
		pr.Post("/auth/2fa/enable", h.HandleEnable2FA)
		pr.Post("/auth/2fa/disable", h.HandleDisable2FA)
		pr.Post("/auth/2fa/verify", h.HandleVerify2FA)
		pr.Get("/auth/2fa/status", h.Handle2FAStatus)

		// Profile
		pr.Get("/auth/profile", h.HandleProfile)
		pr.Post("/auth/profile/update", h.HandleUpdateProfile)
		pr.Patch("/auth/name", h.HandleUpdateName)
		pr.Get("/auth/email/request-change", h.HandleRequestEmailChange)
		pr.Post("/auth/profile/picture", h.UploadProfilePicture)
		pr.Get("/auth/profile/picture/get", h.GetProfilePicture)
		pr.Delete("/auth/profile/picture/remove",h.DeleteProfilePicture)

		// Preferences
		pr.Get("/auth/preferences", h.HandleGetPreferences)
		pr.Post("/auth/preferences/update", h.HandleUpdatePreferences)

		// Password management
		pr.Get("/auth/password/request-change", h.HandleRequestPasswordChange)
		pr.Get("/auth/phone/request-change", h.HandleRequestPhoneChange)
		pr.Get("/auth/phone/request-verification", h.HandleRequestPhoneVerification)
		pr.Get("/auth/email/request-verification", h.HandleRequestEmailVerification)
		pr.Get("/auth/email/get-verification-status",h.HandleGetEmailVerificationStatus)
		pr.Get("/auth/phone/get-verification-status", h.HandleGetPhoneVerificationStatus)


		// Sessions
		pr.Get("/auth/sessions", h.ListSessionsHandler(auth.Client))
		pr.Delete("/auth/logout", h.LogoutHandler(auth.Client))
		pr.Delete("/auth/sessions", h.LogoutAllHandler(auth.Client, rdb))
		pr.Delete("/auth/sessions/{id}", h.DeleteSessionByIDHandler(auth.Client))
	})

	return r
}

