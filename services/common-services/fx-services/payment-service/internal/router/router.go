// internal/router/router. go
package router

import (
	"net/http"
	"time"

	"payment-service/internal/handler"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"go.uber.org/zap"
)

func SetupRoutes(
	webhookHandler *handler.WebhookHandler,
	callbackHandler *handler.CallbackHandler,
	logger *zap.Logger,
) http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(LoggerMiddleware(logger))
	r.Use(middleware.Recoverer)
	r.Use(middleware. Timeout(60 * time.Second))

	// CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-API-Key", "X-Signature", "X-Timestamp"},
		ExposedHeaders:    []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Health check
	r.Get("/api/v1/payments/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		// ============================================
		// WEBHOOKS (Receive from Partner)
		// ============================================
		r.Route("/webhooks", func(r chi.Router) {
			// Specific webhook endpoints
			r.Post("/deposit", webhookHandler.HandleDepositWebhook)
			r.Post("/withdrawal", webhookHandler. HandleWithdrawalWebhook)

			// Catch-all for any other webhook types
			r.Post("/*", webhookHandler.HandleAllWebhooks)
			r.Get("/*", webhookHandler.HandleAllWebhooks)
			r.Put("/*", webhookHandler.HandleAllWebhooks)
		})

		// ============================================
		// CALLBACKS (Receive from Payment Providers)
		// ============================================
		r.Route("/callbacks", func(r chi.Router) {
			// M-Pesa callbacks
			r.Route("/mpesa", func(r chi.Router) {
				// STK Push callbacks (deposits)
				r.Post("/stk/{payment_ref}", callbackHandler. HandleMpesaSTKCallback)

				// B2C callbacks (mobile money withdrawals)
				r.Post("/b2c/{payment_ref}", callbackHandler.HandleMpesaB2CCallback)
				r.Post("/b2c/timeout/{payment_ref}", callbackHandler.HandleMpesaB2CTimeout)

				// ✅ B2B callbacks (bank withdrawals)
				r.Post("/b2b/{payment_ref}", callbackHandler.HandleMpesaB2BCallback)
				r.Post("/b2b/timeout/{payment_ref}", callbackHandler. HandleMpesaB2BTimeout)
			})
		})
	})

	return r
}

// LoggerMiddleware logs HTTP requests
func LoggerMiddleware(logger *zap.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			logger.Info("http request",
				zap.String("method", r.Method),
				zap.String("path", r.URL. Path),
				zap.Int("status", ww.Status()),
				zap.Duration("duration", time.Since(start)),
				zap.String("remote_addr", r.RemoteAddr),
				zap.String("user_agent", r.UserAgent()))
		})
	}
}