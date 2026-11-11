package redis

import (
	"context"
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	ctx := context.Background()
	client, prefix, fastForward := testRedis(t)
	l := NewRateLimiter(client, prefix)

	for i := 0; i < 3; i++ {
		if ok, _, err := l.Allow(ctx, "auth:1.2.3.4", 3, time.Minute); err != nil || !ok {
			t.Fatalf("hit %d: ok=%v err=%v", i+1, ok, err)
		}
	}
	ok, retry, err := l.Allow(ctx, "auth:1.2.3.4", 3, time.Minute)
	if err != nil || ok || retry <= 0 || retry > time.Minute {
		t.Fatalf("4th hit: ok=%v retry=%v err=%v, want blocked with 0<retry<=1m", ok, retry, err)
	}
	if ok, _, _ := l.Allow(ctx, "auth:5.6.7.8", 3, time.Minute); !ok {
		t.Fatal("limits must be per key")
	}

	if fastForward == nil {
		return
	}
	fastForward(time.Minute)
	if ok, _, _ := l.Allow(ctx, "auth:1.2.3.4", 3, time.Minute); !ok {
		t.Fatal("a new window should reset the counter")
	}
}
