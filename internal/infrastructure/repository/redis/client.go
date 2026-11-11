// Package redis holds the Redis-backed repositories. Keys are namespaced with
// a configurable prefix so several environments can share one Redis.
package redis

import (
	"context"
	"fmt"

	goredis "github.com/redis/go-redis/v9"

	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
)

// NewClient connects to the Redis described by url
// (redis://[:password@]host:port/db, or rediss:// for TLS) and pings it so a
// wrong address fails at startup instead of on the first login.
func NewClient(ctx context.Context, url string) (*goredis.Client, error) {
	opts, err := goredis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parse REDIS_URL: %w", err)
	}
	// RESP2 and no CLIENT SETINFO keep older servers (e.g. Redis 3 on
	// Windows dev machines) working; nothing here needs RESP3.
	opts.Protocol = 2
	opts.DisableIndentity = true

	client := goredis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		// opts.Addr only: the URL may contain a password.
		return nil, fmt.Errorf("connect to redis at %s: %w", opts.Addr, err)
	}
	return client, nil
}

type healthChecker struct {
	client goredis.UniversalClient
}

// NewHealthChecker reports Redis in /health.
func NewHealthChecker(client goredis.UniversalClient) serviceinterface.HealthChecker {
	return &healthChecker{client: client}
}

func (h *healthChecker) Name() string { return "redis" }

func (h *healthChecker) Check(ctx context.Context) error {
	return h.client.Ping(ctx).Err()
}
