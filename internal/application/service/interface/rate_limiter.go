package serviceinterface

import (
	"context"
	"time"
)

// RateLimiter counts hits per key in fixed windows. Implementations live in
// infrastructure: an in-memory one for local development and a Redis one that
// is shared by every instance.
type RateLimiter interface {
	// Allow records one hit for key. When the hits in the current window
	// exceed limit it returns allowed=false and the time until the window
	// resets.
	Allow(ctx context.Context, key string, limit int, window time.Duration) (allowed bool, retryAfter time.Duration, err error)
}
