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

func (l *Limiter) CanRequest(ctx context.Context, userID string) error {
	blockKey := fmt.Sprintf("otp:block:%s", userID)
	lastKey := fmt.Sprintf("otp:last:%s", userID)
	countKey := fmt.Sprintf("otp:count:%s", userID)

	// if blocked:
	if n, _ := l.rdb.Exists(ctx, blockKey).Result(); n > 0 {
		return ErrBlocked
	}

	// if last request within cooldown:
	if n, _ := l.rdb.Exists(ctx, lastKey).Result(); n > 0 {
		return ErrTooSoon
	}

	// incr count in window
	cnt, err := l.rdb.Incr(ctx, countKey).Result()
	if err != nil {
		return err
	}
	if cnt == 1 {
		l.rdb.Expire(ctx, countKey, l.window)
	}
	if int(cnt) > l.maxInWindow {
		// block for some time (e.g., window*3)
		l.rdb.Set(ctx, blockKey, "1", l.window*3)
		return ErrBlocked
	}

	// set last request cooldown
	l.rdb.Set(ctx, lastKey, "1", l.cooldown)
	return nil
}
