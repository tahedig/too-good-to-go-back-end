// Package driver opens the PostgreSQL connection pool shared by the
// Postgres repositories.
package driver

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
)

// NewPool connects to the database described by url
// (postgres://user:pass@host:port/db?sslmode=...) and pings it, so a wrong
// address or password fails at startup instead of on the first login.
func NewPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		// pgx redacts the password in parse errors.
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		// Host and database only: the URL may contain a password.
		return nil, fmt.Errorf("connect to postgres at %s:%d/%s: %w",
			cfg.ConnConfig.Host, cfg.ConnConfig.Port, cfg.ConnConfig.Database, err)
	}
	return pool, nil
}

type healthChecker struct {
	pool *pgxpool.Pool
}

// NewHealthChecker reports PostgreSQL in /health.
func NewHealthChecker(pool *pgxpool.Pool) serviceinterface.HealthChecker {
	return &healthChecker{pool: pool}
}

func (h *healthChecker) Name() string { return "postgres" }

func (h *healthChecker) Check(ctx context.Context) error {
	return h.pool.Ping(ctx)
}
