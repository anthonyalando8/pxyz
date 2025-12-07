// middleware/partner_rate_limit.go
package middleware

import (
	//"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// PartnerRateLimit applies rate limiting per partner based on their API settings
func PartnerRateLimit(rdb *redis.Client, logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Get partner from context (injected by RequireAPIKey middleware)
			partner, ok := GetPartnerFromContext(ctx)
			if !ok {
				logger.Error("partner not found in context for rate limiting")
				next.ServeHTTP(w, r)
				return
			}

			// Apply partner-specific rate limit
			limit := int64(partner.APIRateLimit)
			if limit == 0 {
				limit = 100 // Default rate limit
			}

			key := fmt.Sprintf("partner:ratelimit:%s", partner.ID)
			window := time.Minute

			// Check rate limit
			count, err := rdb. Incr(ctx, key).Result()
			if err != nil {
				logger.Error("redis error during rate limiting", zap.Error(err))
				next.ServeHTTP(w, r)
				return
			}

			// Set expiry on first request
			if count == 1 {
				rdb. Expire(ctx, key, window)
			}

			// Check if limit exceeded
			if count > limit {
				ttl, _ := rdb.TTL(ctx, key).Result()
				
				logger. Warn("partner rate limit exceeded",
					zap.String("partner_id", partner.ID),
					zap. Int64("limit", limit),
					zap.Int64("count", count))

				w.Header().Set("X-RateLimit-Limit", fmt. Sprintf("%d", limit))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header(). Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(ttl).Unix()))
				w.Header().Set("Retry-After", fmt.Sprintf("%d", int(ttl.Seconds())))

				render.Status(r, http.StatusTooManyRequests)
				render.JSON(w, r, map[string]interface{}{
					"error":       "rate limit exceeded",
					"limit":       limit,
					"window":      "1 minute",
					"retry_after": int(ttl.Seconds()),
				})
				return
			}

			// Set rate limit headers
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
			w.Header(). Set("X-RateLimit-Remaining", fmt.Sprintf("%d", limit-count))

			next.ServeHTTP(w, r)
		})
	}
}