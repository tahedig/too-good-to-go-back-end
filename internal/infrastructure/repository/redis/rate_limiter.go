package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
)

// rateLimitScript is a fixed-window counter: the first hit creates the key
// with the window as TTL. The PTTL < 0 branch repairs a key that somehow lost
// its expiry, so it can never block a client forever.
// KEYS[1]=key ARGV[1]=window_ms. Returns {hits, ttl_ms}.
var rateLimitScript = goredis.NewScript(`
local n = redis.call('INCR', KEYS[1])
local ttl = redis.call('PTTL', KEYS[1])
if n == 1 or ttl < 0 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
  ttl = tonumber(ARGV[1])
end
return {n, ttl}
`)

type rateLimiter struct {
	client goredis.UniversalClient
	prefix string
}

// NewRateLimiter returns a limiter whose counters are shared by every API
// instance using the same Redis.
func NewRateLimiter(client goredis.UniversalClient, keyPrefix string) serviceinterface.RateLimiter {
	return &rateLimiter{client: client, prefix: keyPrefix + "ratelimit:"}
}

func (l *rateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, time.Duration, error) {
	res, err := rateLimitScript.Run(ctx, l.client, []string{l.prefix + key}, window.Milliseconds()).Int64Slice()
	if err != nil {
		return false, 0, fmt.Errorf("redis rate limit: %w", err)
	}
	if len(res) != 2 {
		return false, 0, fmt.Errorf("redis rate limit: unexpected reply %v", res)
	}
	hits, ttl := res[0], time.Duration(res[1])*time.Millisecond
	if hits > int64(limit) {
		return false, ttl, nil
	}
	return true, 0, nil
}
