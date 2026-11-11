package memory

import (
	"context"
	"sync"
	"time"

	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
)

type windowCounter struct {
	start  time.Time
	window time.Duration
	count  int
}

type rateLimiter struct {
	now func() time.Time

	mu        sync.Mutex
	counters  map[string]*windowCounter
	lastSweep time.Time
}

// NewRateLimiter returns a fixed-window limiter. Counters are per process, so
// with several instances each one enforces its own limit.
func NewRateLimiter() serviceinterface.RateLimiter { return newRateLimiter(time.Now) }

func newRateLimiter(now func() time.Time) *rateLimiter {
	return &rateLimiter{now: now, counters: make(map[string]*windowCounter), lastSweep: now()}
}

func (l *rateLimiter) Allow(_ context.Context, key string, limit int, window time.Duration) (bool, time.Duration, error) {
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()

	// Drop finished windows at most once per window so memory stays bounded
	// by the number of clients seen recently.
	if now.Sub(l.lastSweep) >= window {
		for k, wc := range l.counters {
			if now.Sub(wc.start) >= wc.window {
				delete(l.counters, k)
			}
		}
		l.lastSweep = now
	}

	wc, ok := l.counters[key]
	if !ok || now.Sub(wc.start) >= wc.window {
		l.counters[key] = &windowCounter{start: now, window: window, count: 1}
		return true, 0, nil
	}
	if wc.count >= limit {
		return false, wc.start.Add(wc.window).Sub(now), nil
	}
	wc.count++
	return true, 0, nil
}
