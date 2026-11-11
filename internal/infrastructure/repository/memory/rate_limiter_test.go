package memory

import (
	"context"
	"sync"
	"testing"
	"time"
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func allow(l *rateLimiter, key string) (bool, time.Duration) {
	ok, retry, _ := l.Allow(context.Background(), key, 3, time.Minute)
	return ok, retry
}

func TestRateLimiterWindow(t *testing.T) {
	clock := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	l := newRateLimiter(clock.now)

	for i := 0; i < 3; i++ {
		if ok, _ := allow(l, "1.2.3.4"); !ok {
			t.Fatalf("hit %d should be allowed", i+1)
		}
	}
	clock.advance(20 * time.Second)
	if ok, retry := allow(l, "1.2.3.4"); ok || retry != 40*time.Second {
		t.Fatalf("4th hit: ok=%v retry=%v, want blocked for 40s", ok, retry)
	}
	if ok, _ := allow(l, "5.6.7.8"); !ok {
		t.Fatal("limits must be per key")
	}

	clock.advance(40 * time.Second)
	if ok, _ := allow(l, "1.2.3.4"); !ok {
		t.Fatal("a new window should reset the counter")
	}
}

func TestRateLimiterSweepsOldKeys(t *testing.T) {
	clock := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	l := newRateLimiter(clock.now)
	for _, k := range []string{"a", "b", "c"} {
		allow(l, k)
	}
	clock.advance(2 * time.Minute)
	allow(l, "d")
	if n := len(l.counters); n != 1 {
		t.Fatalf("expired windows should be swept, %d counters left", n)
	}
}

func TestRateLimiterConcurrent(t *testing.T) {
	l := newRateLimiter(time.Now)
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		allowed int
	)
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ok, _, _ := l.Allow(context.Background(), "k", 50, time.Hour); ok {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if allowed != 50 {
		t.Fatalf("allowed %d requests, want exactly 50", allowed)
	}
}
