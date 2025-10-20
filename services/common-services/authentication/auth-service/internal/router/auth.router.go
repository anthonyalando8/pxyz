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
	"x/shared/utils/cache"
)

func SetupRoutes(
	r chi.Router,
	h *handler.AuthHandler,
	oauthHandler *handler.OAuth2Handler,
	auth *middleware.MiddlewareWithClient,
	wsHandler *handler.WSHandler,
	cache *cache.Cache,
	rdb *redis.Client,
) chi.Router {
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "ngrok-skip-browser-warning"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	r.Use(auth.RateLimit(rdb, 100, time.Minute, time.Minute, "global_user_auth"))

	uploadDir := "/app/uploads"
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		_ = os.MkdirAll(uploadDir, 0755)
	}

	// ================================
	// OAUTH2 PUBLIC ENDPOINTS (Outside /api/v1)
	// ================================
	r.Route("/oauth2", func(oauth chi.Router) {
		// Authorization endpoint - public
		oauth.Get("/authorize", oauthHandler.Authorize)
		
		// Token endpoint - public (client authenticates via credentials)
		oauth.Post("/token", oauthHandler.Token)
		
		// Token revocation - public
		oauth.Post("/revoke", oauthHandler.Revoke)
		
		// Token introspection - public (requires client auth)
		oauth.Post("/introspect", oauthHandler.Introspect)
		
		// Consent endpoints - require user authentication
		oauth.Group(func(consent chi.Router) {
			consent.Use(auth.Require([]string{"main", "temp"}, nil, nil))
			consent.Get("/consent", oauthHandler.ShowConsent)
			consent.Post("/consent", oauthHandler.GrantConsent)
		})
	})

	r.Route("/api/v1", func(api chi.Router) {
		api.Use(auth.RateLimit(rdb, 5, 30*time.Second, 30*time.Second, "user_auth"))
		
		// ---------------- Public ----------------
		api.Group(func(pub chi.Router) {
			pub.Get("/auth/health", h.Health)
			pub.Post("/auth/submit-identifier", h.SubmitIdentifier)
			pub.Post("/auth/google", h.GoogleAuthHandler)
			pub.Post("/auth/telegram", h.TelegramLogin)
			pub.Post("/auth/apple", h.AppleAuthHandler)
			pub.Post("/auth/password/forgot", h.HandleForgotPassword)
			pub.Handle("/auth/uploads/*", http.StripPrefix("/auth/uploads/", http.FileServer(http.Dir(uploadDir))))
		})

		// ---------------- Account Initialization ----------------
		api.Group(func(g chi.Router) {
			g.Use(auth.Require([]string{"temp"}, []string{"init_account"}, nil))
			g.Post("/auth/verify-identifier", h.VerifyIdentifier)
			g.Post("/auth/set-password", h.SetPassword)
			g.Post("/auth/login-password", h.LoginWithPassword)
			g.Get("/auth/cached-status", h.GetCachedUserStatus)
			g.Get("/auth/resend-identifier-code", h.ResendOTP)
		})

		// ---------------- Password Reset ----------------
		api.Group(func(g chi.Router) {
			g.Use(auth.Require([]string{"temp"}, []string{"password_reset"}, nil))
			g.Post("/auth/password/reset", h.HandleResetPassword)
		})

		// ---------------- Registration & OTP ----------------
		api.Group(func(g chi.Router) {
			g.Use(auth.Require([]string{"temp", "main"}, []string{"register", "email_change", "incomplete_profile", "general", "verify-otp", "phone_change"}, nil))
			g.Post("/auth/register/otp/request", h.HandleRequestOTP)
			g.Post("/auth/register/otp/verify", h.HandleVerifyOTP)
		})

		// ---------------- Email & Phone Change ----------------
		api.Group(func(g chi.Router) {
			g.Use(auth.Require([]string{"temp", "main"}, []string{"email_change"}, nil))
			g.Patch("/auth/email", h.HandleChangeEmail)
		})
		api.Group(func(g chi.Router) {
			g.Use(auth.Require([]string{"temp", "main"}, []string{"phone_change"}, nil))
			g.Patch("/auth/phone/update", h.HandlePhoneChange)
		})

		// ---------------- Profile Completion ----------------
		api.Group(func(g chi.Router) {
			g.Use(auth.Require([]string{"temp", "main"}, []string{"general", "register", "incomplete_profile"}, nil))
			g.Post("/auth/profile/nationality", h.HandleUpdateNationality)
		})

		// ---------------- Authenticated User ----------------
		api.Group(func(g chi.Router) {
			g.Use(auth.Require([]string{"main"}, nil, nil))

			g.Get("/auth/ws", wsHandler.HandleWS)

			g.Route("/auth/2fa", func(r chi.Router) {
				r.Get("/init", h.HandleInitiate2FA)
				r.Post("/enable", h.HandleEnable2FA)
				r.Post("/disable", h.HandleDisable2FA)
				r.Post("/verify", h.HandleVerify2FA)
				r.Get("/status", h.Handle2FAStatus)
			})

			g.Route("/auth/profile", func(r chi.Router) {
				r.Get("/", h.HandleProfile)
				r.Post("/update", h.HandleUpdateProfile)
				r.Get("/picture/get", h.GetProfilePicture)
				r.Post("/picture", h.UploadProfilePicture)
				r.Delete("/picture/remove", h.DeleteProfilePicture)
				r.Get("/email/request-change", h.HandleRequestEmailChange)
			})

			g.Route("/auth/preferences", func(r chi.Router) {
				r.Get("/", h.HandleGetPreferences)
				r.Post("/update", h.HandleUpdatePreferences)
			})

			g.Route("/auth/password", func(r chi.Router) {
				r.Get("/request-change", h.HandleRequestPasswordChange)
			})

			g.Route("/auth/phone", func(r chi.Router) {
				r.Get("/request-change", h.HandleRequestPhoneChange)
				r.Get("/request-verification", h.HandleRequestPhoneVerification)
				r.Get("/get-verification-status", h.HandleGetPhoneVerificationStatus)
			})

			g.Route("/auth/email", func(r chi.Router) {
				r.Get("/request-verification", h.HandleRequestEmailVerification)
				r.Get("/get-verification-status", h.HandleGetEmailVerificationStatus)
			})

			g.Route("/auth/sessions", func(r chi.Router) {
				r.Get("/", h.ListSessionsHandler(auth.Client))
				r.Delete("/", h.LogoutAllHandler(auth.Client, rdb))
				r.Delete("/{id}", h.DeleteSessionByIDHandler(auth.Client))
			})
			g.Delete("/auth/logout", h.LogoutHandler(auth.Client))

			// ================================
			// OAUTH2 CLIENT MANAGEMENT (Authenticated)
			// ================================
			g.Route("/oauth2/clients", func(oauth chi.Router) {
				oauth.Post("/", oauthHandler.RegisterClient)
				oauth.Get("/", oauthHandler.ListMyClients)
				oauth.Get("/{client_id}", oauthHandler.GetClient)
				oauth.Put("/{client_id}", oauthHandler.UpdateClient)
				oauth.Delete("/{client_id}", oauthHandler.DeleteClient)
				oauth.Post("/{client_id}/regenerate-secret", oauthHandler.RegenerateClientSecret)
			})

			// ================================
			// USER CONSENT MANAGEMENT (Authenticated)
			// ================================
			g.Route("/oauth2/consents", func(consent chi.Router) {
				consent.Get("/", oauthHandler.ListMyConsents)
				consent.Delete("/", oauthHandler.RevokeAllConsents)
				consent.Delete("/{client_id}", oauthHandler.RevokeConsent)
			})
		})
	})

	return r
}