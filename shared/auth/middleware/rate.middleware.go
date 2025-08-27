package middleware

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"x/shared/response"

	"github.com/redis/go-redis/v9"
)

func RateLimiter(rdb *redis.Client, limit int, window, blockDuration time.Duration, keyPrefix string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.Background()

			// 1. Prefer userID if authenticated
			var clientID string
			userID := r.Context().Value(ContextUserID)
			if userIDStr, ok := userID.(string); ok && userIDStr != "" {
				clientID = "uid:" + userIDStr
			} else {
				// 2. Fallback: IP (check proxy headers first)
				ip := r.Header.Get("X-Forwarded-For")
				if ip == "" {
					ip = r.RemoteAddr
				}
				clientID = "ip:" + strings.Split(ip, ",")[0]
			}

			key := keyPrefix + ":" + clientID
			blockKey := key + ":blocked"

			// Check if already blocked
			blocked, _ := rdb.Get(ctx, blockKey).Result()
			if blocked == "1" {
				ttl, _ := rdb.TTL(ctx, blockKey).Result()
				w.Header().Set("Retry-After", strconv.Itoa(int(ttl.Seconds())))
				response.Error(w, http.StatusTooManyRequests,"Too Many Requests. Try again in "+ttl.String())
				return
			}

			// Increment counter
			count, err := rdb.Incr(ctx, key).Result()
			if err != nil {
				// Fail open → don’t block traffic if Redis unavailable
				next.ServeHTTP(w, r)
				return
			}

			// First request → set expiry
			if count == 1 {
				rdb.Expire(ctx, key, window)
			}

			// Over the limit? → block
			if count > int64(limit) {
				rdb.Set(ctx, blockKey, "1", blockDuration)
				w.Header().Set("Retry-After", strconv.Itoa(int(blockDuration.Seconds())))
				response.Error(w, http.StatusTooManyRequests,"Too Many Requests. Blocked for "+blockDuration.String())
				return
			}

			//  Optional headers
			ttl, _ := rdb.TTL(ctx, key).Result()
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(limit-int(count)))
			w.Header().Set("X-RateLimit-Reset", strconv.Itoa(int(ttl.Seconds())))

			next.ServeHTTP(w, r)
		})
	}
}
