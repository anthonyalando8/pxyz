package rate

import (
	"context"
	"errors"
	"fmt"
	"time"

	"x/shared/utils/cache"
)

type Limiter struct {
	cache *cache.Cache
	window     time.Duration
	maxInWindow int
	cooldown   time.Duration
}

func NewLimiter(cache *cache.Cache, window time.Duration, max int, cooldown time.Duration) *Limiter {
	return &Limiter{cache: cache, window: window, maxInWindow: max, cooldown: cooldown}
}

var (
	ErrTooSoon = errors.New("please wait before requesting another OTP")
	ErrBlocked = errors.New("too many OTP requests; try again later")
)

func (l *Limiter) CanRequest(ctx context.Context, userID, purpose string) error {
	blockKey := fmt.Sprintf("otp:block:%s:%s", userID, purpose)
	lastKey := fmt.Sprintf("otp:last:%s:%s", userID, purpose)
	countKey := fmt.Sprintf("otp:count:%s:%s", userID, purpose)

	// 1️⃣ Check block (too many requests in window)
	if ttl, _ := l.cache.GetTTL(ctx,"otp_rate", blockKey); ttl > 0 {
		return fmt.Errorf("too many OTP requests; please try again after %d seconds", int(ttl.Seconds()))
	}

	// 2️⃣ Check last request cooldown
	if ttl, _ := l.cache.GetTTL(ctx,"otp_rate", lastKey); ttl > 0 {
		return fmt.Errorf("please wait %d seconds before requesting another OTP", int(ttl.Seconds()))
	}

	// 3️⃣ Increment count within window
	cnt, err := l.cache.IncrWithExpire(ctx,"otp_rate", countKey, l.window)
	if err != nil {
		return err
	}

	if int(cnt) > l.maxInWindow {
		// too many requests → block for extended time
		_ = l.cache.Set(ctx,"otp_rate", blockKey, "1", l.window*3)
		return fmt.Errorf("too many OTP requests; please try again after %d seconds", int((l.window*3).Seconds()))
	}

	// Set cooldown for last request
	_ = l.cache.Set(ctx, "otp_rate", lastKey, "1", l.cooldown)

	return nil
}

