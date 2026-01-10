// middleware/api_key_auth.go
package middleware

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"partner-service/internal/domain"
	"partner-service/internal/repository"

	"github.com/go-chi/render"
	"go.uber.org/zap"
)

type contextKey string

const (
	PartnerContextKey   contextKey = "partner"
	PartnerIDKey        contextKey = "partner_id"
	ClientIPKey         contextKey = "client_ip"           // ✅ NEW
	UserAgentKey        contextKey = "user_agent"          // ✅ NEW
	RequestStartTimeKey contextKey = "request_start_time"  // ✅ NEW
	ErrorMessageKey     contextKey = "error_message"       // ✅ NEW
	RequestLatencyKey   contextKey = "request_latency"     // ✅ NEW
)

type APIKeyAuthMiddleware struct {
	partnerRepo *repository. PartnerRepo
	logger      *zap.Logger
}

func NewAPIKeyAuthMiddleware(partnerRepo *repository.PartnerRepo, logger *zap.Logger) *APIKeyAuthMiddleware {
	return &APIKeyAuthMiddleware{
		partnerRepo:  partnerRepo,
		logger:       logger,
	}
}

// ✅ NEW: Response writer wrapper to capture status code
type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *statusResponseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *statusResponseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// RequireAPIKey validates API key and secret, then injects partner into context
func (m *APIKeyAuthMiddleware) RequireAPIKey() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// ✅ Capture request start time
			startTime := time.Now()
			
			ctx := r.Context()
			
			// ✅ Extract and store client IP
			clientIP := getClientIP(r)
			ctx = context.WithValue(ctx, ClientIPKey, clientIP)
			
			// ✅ Extract and store user agent
			userAgent := r.UserAgent()
			ctx = context.WithValue(ctx, UserAgentKey, userAgent)
			
			// ✅ Store request start time
			ctx = context.WithValue(ctx, RequestStartTimeKey, startTime)

			// Extract API credentials from headers
			apiKey := r.Header.Get("X-API-Key")
			apiSecret := r.Header.Get("X-API-Secret")

			if apiKey == "" || apiSecret == "" {
				m. logger. Warn("missing API credentials",
					zap.String("path", r.URL.Path),
					zap.String("ip", clientIP))
				m.sendErrorWithContext(w, r, ctx, http.StatusUnauthorized, "missing API credentials")
				return
			}

			// Fetch partner by API key
			partner, err := m.partnerRepo. GetPartnerByAPIKey(ctx, apiKey)
			if err != nil {
				m.logger.Warn("invalid API key",
					zap.String("api_key", maskAPIKey(apiKey)),
					zap.String("ip", clientIP),
					zap.Error(err))
				m.sendErrorWithContext(w, r, ctx, http.StatusUnauthorized, "invalid API credentials")
				return
			}

			// Verify partner status
			if partner.Status != "active" {
				m.logger. Warn("inactive partner attempted access",
					zap.String("partner_id", partner.ID),
					zap.String("status", string(partner.Status)))
				m.sendErrorWithContext(w, r, ctx, http.StatusForbidden, "partner account is not active")
				return
			}

			// Verify API is enabled
			if !partner.IsAPIEnabled {
				m.logger.Warn("API access disabled for partner",
					zap.String("partner_id", partner.ID))
				m.sendErrorWithContext(w, r, ctx, http.StatusForbidden, "API access is disabled")
				return
			}

			// Verify API secret
			if ! m.verifyAPISecret(apiSecret, partner.APISecretHash) {
				m.logger.Warn("invalid API secret",
					zap.String("partner_id", partner.ID),
					zap.String("ip", clientIP))
				m.sendErrorWithContext(w, r, ctx, http.StatusUnauthorized, "invalid API credentials")
				return
			}

			// Check IP whitelist if configured
			if len(partner.AllowedIPs) > 0 {
				if !m.isIPAllowed(clientIP, partner.AllowedIPs) {
					m.logger.Warn("IP not whitelisted",
						zap. String("partner_id", partner. ID),
						zap.String("ip", clientIP),
						zap.Strings("allowed_ips", partner.AllowedIPs))
					m.sendErrorWithContext(w, r, ctx, http.StatusForbidden, "IP address not authorized")
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

			// ✅ Wrap response writer to capture status code
			wrappedWriter := &statusResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Continue to next handler with updated context
			next.ServeHTTP(wrappedWriter, r.WithContext(ctx))

			// ✅ Calculate and store latency after request completes
			latencyMs := time.Since(startTime).Milliseconds()
			// Note: We can't update context after the handler runs, but handlers can access startTime
			
			// ✅ Log request completion
			m.logger.Debug("request completed",
				zap.String("partner_id", partner.ID),
				zap.String("endpoint", r.URL.Path),
				zap.Int("status_code", wrappedWriter.statusCode),
				zap.Int64("latency_ms", latencyMs))
		})
	}
}

// ✅ NEW: sendErrorWithContext stores error message in context before sending response
func (m *APIKeyAuthMiddleware) sendErrorWithContext(w http.ResponseWriter, r *http.Request, ctx context.Context, status int, message string) {
	// Store error message in context for logging
	ctx = context.WithValue(ctx, ErrorMessageKey, message)
	
	render.Status(r, status)
	render.JSON(w, r, map[string]interface{}{
		"error":     message,
		"timestamp": time.Now().Unix(),
	})
}

// verifyAPISecret compares the provided secret with the stored hash
func (m *APIKeyAuthMiddleware) verifyAPISecret(providedSecret string, storedHash *string) bool {
	if storedHash == nil || *storedHash == "" {
		return false
	}

	// Hash the provided secret with SHA256 (same as generation)
	hash := sha256.Sum256([]byte(providedSecret))
	providedHash := hex.EncodeToString(hash[:])

	// Constant-time comparison
	return subtle. ConstantTimeCompare([]byte(providedHash), []byte(*storedHash)) == 1
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
		"timestamp": time. Now().Unix(),
	})
}

// Helper functions

func getClientIP(r *http.Request) string {
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

// ✅ Context helper functions

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

// ✅ NEW: GetClientIPFromContext retrieves client IP from context
func GetClientIPFromContext(ctx context.Context) string {
	if ip, ok := ctx.Value(ClientIPKey).(string); ok {
		return ip
	}
	return ""
}

// ✅ NEW: GetUserAgentFromContext retrieves user agent from context
func GetUserAgentFromContext(ctx context.Context) string {
	if ua, ok := ctx.Value(UserAgentKey).(string); ok {
		return ua
	}
	return ""
}

// ✅ NEW: GetRequestLatency calculates latency from start time in context
func GetRequestLatency(ctx context.Context) int64 {
	if startTime, ok := ctx.Value(RequestStartTimeKey).(time.Time); ok {
		return time.Since(startTime).Milliseconds()
	}
	return 0
}

// ✅ NEW: GetErrorMessageFromContext retrieves error message from context
func GetErrorMessageFromContext(ctx context.Context) string {
	if errMsg, ok := ctx.Value(ErrorMessageKey).(string); ok {
		return errMsg
	}
	return ""
}

// MustGetPartnerFromContext retrieves partner or panics (use in handlers after middleware)
func MustGetPartnerFromContext(ctx context. Context) *domain.Partner {
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