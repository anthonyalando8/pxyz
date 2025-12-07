// middleware/api_key_auth.go
package middleware

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"

	//"fmt"
	"net/http"
	"strings"
	"time"

	"partner-service/internal/domain"
	"partner-service/internal/repository"

	"github.com/go-chi/render"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const (
	PartnerContextKey contextKey = "partner"
	PartnerIDKey      contextKey = "partner_id"
)

type APIKeyAuthMiddleware struct {
	partnerRepo *repository.PartnerRepo
	logger      *zap. Logger
}

func NewAPIKeyAuthMiddleware(partnerRepo *repository.PartnerRepo, logger *zap.Logger) *APIKeyAuthMiddleware {
	return &APIKeyAuthMiddleware{
		partnerRepo: partnerRepo,
		logger:      logger,
	}
}

// RequireAPIKey validates API key and secret, then injects partner into context
func (m *APIKeyAuthMiddleware) RequireAPIKey() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Extract API credentials from headers
			apiKey := r.Header.Get("X-API-Key")
			apiSecret := r.Header.Get("X-API-Secret")

			if apiKey == "" || apiSecret == "" {
				m. logger. Warn("missing API credentials",
					zap.String("path", r.URL.Path),
					zap.String("ip", getClientIP(r)))
				m.sendError(w, r, http. StatusUnauthorized, "missing API credentials")
				return
			}

			// Fetch partner by API key
			partner, err := m.partnerRepo.GetPartnerByAPIKey(ctx, apiKey)
			if err != nil {
				m.logger. Warn("invalid API key",
					zap. String("api_key", maskAPIKey(apiKey)),
					zap.String("ip", getClientIP(r)),
					zap.Error(err))
				m.sendError(w, r, http.StatusUnauthorized, "invalid API credentials")
				return
			}

			// Verify partner status
			if partner.Status != "active" {
				m.logger.Warn("inactive partner attempted access",
					zap.String("partner_id", partner.ID),
					zap.String("status", string(partner.Status)))
				m.sendError(w, r, http.StatusForbidden, "partner account is not active")
				return
			}

			// Verify API is enabled
			if !partner.IsAPIEnabled {
				m.logger.Warn("API access disabled for partner",
					zap.String("partner_id", partner.ID))
				m.sendError(w, r, http.StatusForbidden, "API access is disabled")
				return
			}

			// Verify API secret
			if ! m.verifyAPISecret(apiSecret, partner.APISecretHash) {
				m.logger. Warn("invalid API secret",
					zap.String("partner_id", partner.ID),
					zap.String("ip", getClientIP(r)))
				m.sendError(w, r, http.StatusUnauthorized, "invalid API credentials")
				return
			}

			// Check IP whitelist if configured
			if len(partner.AllowedIPs) > 0 {
				clientIP := getClientIP(r)
				if !m.isIPAllowed(clientIP, partner.AllowedIPs) {
					m.logger.Warn("IP not whitelisted",
						zap. String("partner_id", partner. ID),
						zap.String("ip", clientIP),
						zap. Strings("allowed_ips", partner.AllowedIPs))
					m.sendError(w, r, http.StatusForbidden, "IP address not authorized")
					return
				}
			}

			// Log successful authentication
			m.logger.Info("API authentication successful",
				zap.String("partner_id", partner.ID),
				zap.String("partner_name", partner.Name),
				zap.String("endpoint", r.URL.Path),
				zap.String("method", r.Method))

			// Inject partner into context
			ctx = context.WithValue(ctx, PartnerContextKey, partner)
			ctx = context.WithValue(ctx, PartnerIDKey, partner.ID)

			// Continue to next handler
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// verifyAPISecret compares the provided secret with the stored hash
func (m *APIKeyAuthMiddleware) verifyAPISecret(providedSecret string, storedHash *string) bool {
	if storedHash == nil || *storedHash == "" {
		return false
	}

	// Hash the provided secret with SHA256 (same as generation)
	hash := sha256. Sum256([]byte(providedSecret))
	providedHash := hex.EncodeToString(hash[:])

	// Constant-time comparison
	return subtle.ConstantTimeCompare([]byte(providedHash), []byte(*storedHash)) == 1
}

// isIPAllowed checks if client IP is in whitelist
func (m *APIKeyAuthMiddleware) isIPAllowed(clientIP string, allowedIPs []string) bool {
	if len(allowedIPs) == 0 {
		return true // No restriction
	}

	for _, allowedIP := range allowedIPs {
		if clientIP == allowedIP || allowedIP == "*" {
			return true
		}
		// Support CIDR notation in the future
		// if matchesCIDR(clientIP, allowedIP) { return true }
	}

	return false
}

// sendError sends a JSON error response
func (m *APIKeyAuthMiddleware) sendError(w http.ResponseWriter, r *http.Request, status int, message string) {
	render.Status(r, status)
	render.JSON(w, r, map[string]interface{}{
		"error":     message,
		"timestamp": time.Now().Unix(),
	})
}

// Helper functions

func getClientIP(r *http. Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header. Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check X-Real-IP header
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return xrip
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if colonIdx := strings.LastIndex(ip, ":"); colonIdx != -1 {
		ip = ip[:colonIdx]
	}
	return ip
}

func maskAPIKey(apiKey string) string {
	if len(apiKey) <= 8 {
		return "***"
	}
	return apiKey[:4] + "..." + apiKey[len(apiKey)-4:]
}

// GetPartnerFromContext retrieves partner from request context
func GetPartnerFromContext(ctx context.Context) (*domain.Partner, bool) {
	partner, ok := ctx.Value(PartnerContextKey).(*domain.Partner)
	return partner, ok
}

// GetPartnerIDFromContext retrieves partner ID from request context
func GetPartnerIDFromContext(ctx context.Context) (string, bool) {
	partnerID, ok := ctx.Value(PartnerIDKey).(string)
	return partnerID, ok
}

// MustGetPartnerFromContext retrieves partner or panics (use in handlers after middleware)
func MustGetPartnerFromContext(ctx context.Context) *domain.Partner {
	partner, ok := GetPartnerFromContext(ctx)
	if !ok {
		panic("partner not found in context - ensure RequireAPIKey middleware is applied")
	}
	return partner
}

// MustGetPartnerIDFromContext retrieves partner ID or panics
func MustGetPartnerIDFromContext(ctx context.Context) string {
	partnerID, ok := GetPartnerIDFromContext(ctx)
	if !ok {
		panic("partner ID not found in context - ensure RequireAPIKey middleware is applied")
	}
	return partnerID
}