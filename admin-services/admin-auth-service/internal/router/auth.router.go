package router

import (
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/redis/go-redis/v9"

	"admin-auth-service/internal/handler"
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
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "ngrok-skip-browser-warning"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	r.Use(auth.RateLimit(rdb, 100, time.Minute, 10*time.Minute, "global"))

	uploadDir := "/app/uploads"
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		os.MkdirAll(uploadDir, 0755)
	}

	r.Route("/admin/auth", func(r chi.Router) {

		// ---------------- Public ----------------
		r.Group(func(r chi.Router) {
			r.Use(auth.RateLimit(rdb, 5, 30*time.Second, 30*time.Second, "auth"))
			r.Post("/exists", h.HandleUserExists)
			r.Post("/login", h.HandleLogin)
			r.Post("/password/forgot", h.HandleForgotPassword)
		})

		// ---------------- Registration & Password Flows ----------------
		r.Group(func(r chi.Router) {
			r.Use(auth.Require([]string{"main", "temp"}, []string{"register"}, nil))
			r.Post("/password/set", h.HandleSetPassword)
		})

		r.Group(func(r chi.Router) {
			r.Use(auth.Require([]string{"temp"}, []string{"password_reset"}, nil))
			r.Post("/password/reset", h.HandleResetPassword)
		})

		r.Group(func(r chi.Router) {
			r.Use(auth.Require([]string{"main", "temp"}, []string{
				"register", "email_change", "incomplete_profile", "general",
				"verify-otp", "phone_change",
			}, nil))
			r.Post("/register/otp/request", h.HandleRequestOTP)
			r.Post("/register/otp/verify", h.HandleVerifyOTP)
		})

		// Email & Phone Change (OTP-protected)
		r.Group(func(r chi.Router) {
			r.Use(auth.Require([]string{"main", "temp"}, []string{"email_change"}, nil))
			r.Patch("/email", h.HandleChangeEmail)
		})
		r.Group(func(r chi.Router) {
			r.Use(auth.Require([]string{"main", "temp"}, []string{"phone_change"}, nil))
			r.Patch("/phone/update", h.HandlePhoneChange)
		})

		// ---------------- Authenticated User Endpoints ----------------
		r.Group(func(pr chi.Router) {
			pr.Use(auth.Require([]string{"main"}, nil, []string{"super_admin"})) // main session required, no role enforced

			// Serve uploads
			pr.Handle("/uploads/*", http.StripPrefix("/admin/auth/uploads/", http.FileServer(http.Dir(uploadDir))))
			// WebSocket
			pr.Get("/ws", wsHandler.HandleWS)

			pr.Post("/register", h.HandleRegister)

			// 2FA
			pr.Get("/2fa/init", h.HandleInitiate2FA)
			pr.Post("/2fa/enable", h.HandleEnable2FA)
			pr.Post("/2fa/disable", h.HandleDisable2FA)
			pr.Post("/2fa/verify", h.HandleVerify2FA)
			pr.Get("/2fa/status", h.Handle2FAStatus)

			// Profile
			pr.Get("/profile", h.HandleProfile)
			pr.Post("/profile/update", h.HandleUpdateProfile)
			pr.Patch("/name", h.HandleUpdateName)
			pr.Get("/email/request-change", h.HandleRequestEmailChange)
			pr.Post("/profile/picture", h.UploadProfilePicture)

			// Preferences
			pr.Get("/preferences", h.HandleGetPreferences)
			pr.Post("/preferences/update", h.HandleUpdatePreferences)

			// Password management
			pr.Get("/password/request-change", h.HandleRequestPasswordChange)
			pr.Get("/phone/request-change", h.HandleRequestPhoneChange)
			pr.Get("/phone/request-verification", h.HandleRequestPhoneVerification)
			pr.Get("/email/request-verification", h.HandleRequestEmailVerification)

			// Sessions
			pr.Get("/sessions", h.ListSessionsHandler(auth.Client))
			pr.Delete("/logout", h.LogoutHandler(auth.Client))
			pr.Delete("/sessions", h.LogoutAllHandler(auth.Client, rdb))
			pr.Delete("/sessions/{id}", h.DeleteSessionByIDHandler(auth.Client))
		})
	})

	return r
}


