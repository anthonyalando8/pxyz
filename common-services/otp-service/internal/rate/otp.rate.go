package rate

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Limiter struct {
	rdb        *redis.Client
	window     time.Duration
	maxInWindow int
	cooldown   time.Duration
}

func NewLimiter(rdb *redis.Client, window time.Duration, max int, cooldown time.Duration) *Limiter {
	return &Limiter{rdb: rdb, window: window, maxInWindow: max, cooldown: cooldown}
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
	if ttl, _ := l.rdb.TTL(ctx, blockKey).Result(); ttl > 0 {
		return fmt.Errorf("too many OTP requests; please try again after %d seconds", int(ttl.Seconds()))
	}

	// 2️⃣ Check last request cooldown
	if ttl, _ := l.rdb.TTL(ctx, lastKey).Result(); ttl > 0 {
		return fmt.Errorf("please wait %d seconds before requesting another OTP", int(ttl.Seconds()))
	}

	// 3️⃣ Increment count within window
	cnt, err := l.rdb.Incr(ctx, countKey).Result()
	if err != nil {
		return err
	}
	if cnt == 1 {
		// first request, set expiration
		_ = l.rdb.Expire(ctx, countKey, l.window).Err()
	}

	if int(cnt) > l.maxInWindow {
		// too many requests → block for extended time
		_ = l.rdb.Set(ctx, blockKey, "1", l.window*3).Err()
		return fmt.Errorf("too many OTP requests; please try again after %d seconds", int((l.window*3).Seconds()))
	}

	// Set cooldown for last request
	_ = l.rdb.Set(ctx, lastKey, "1", l.cooldown).Err()

	return nil
}

