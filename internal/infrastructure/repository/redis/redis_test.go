package redis

import (
	"context"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
)

const phone = "+989123456789"

// testRedis returns a client and a unique key prefix. By default it uses an
// in-process miniredis; set REDIS_TEST_URL (e.g. redis://localhost:6379/15)
// to run the same tests against a real server. fastForward is nil for a real
// server, where tests that need time travel are skipped.
func testRedis(t *testing.T) (client *goredis.Client, prefix string, fastForward func(time.Duration)) {
	t.Helper()
	prefix = "test:" + strconv.FormatInt(time.Now().UnixNano(), 36) + ":"

	if url := os.Getenv("REDIS_TEST_URL"); url != "" {
		c, err := NewClient(context.Background(), url)
		if err != nil {
			t.Fatalf("REDIS_TEST_URL: %v", err)
		}
		t.Cleanup(func() {
			ctx := context.Background()
			keys, _ := c.Keys(ctx, prefix+"*").Result()
			if len(keys) > 0 {
				c.Del(ctx, keys...)
			}
			_ = c.Close()
		})
		return c, prefix, nil
	}

	mr := miniredis.RunT(t)
	c, err := NewClient(context.Background(), "redis://"+mr.Addr())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c, prefix, mr.FastForward
}

func TestOTPSaveAndCooldown(t *testing.T) {
	ctx := context.Background()
	client, prefix, _ := testRedis(t)
	repo := NewOTPRepository(client, prefix)
	t0 := time.UnixMilli(time.Now().UnixMilli())

	first := &model.OTP{Phone: phone, CodeHash: "h1", Salt: "s1", CreatedAt: t0, ExpiresAt: t0.Add(5 * time.Minute)}
	if blocked, err := repo.Save(ctx, first, time.Minute); err != nil || blocked != nil {
		t.Fatalf("first save: blocked=%+v err=%v", blocked, err)
	}

	second := &model.OTP{Phone: phone, CodeHash: "h2", Salt: "s2", CreatedAt: t0.Add(30 * time.Second), ExpiresAt: t0.Add(6 * time.Minute)}
	blocked, err := repo.Save(ctx, second, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if blocked == nil || blocked.CodeHash != "h1" || blocked.Salt != "s1" || !blocked.CreatedAt.Equal(t0) ||
		!blocked.ExpiresAt.Equal(first.ExpiresAt) || blocked.Attempts != 0 || blocked.Used {
		t.Fatalf("save inside cooldown should return the stored otp unchanged, got %+v", blocked)
	}

	third := &model.OTP{Phone: phone, CodeHash: "h3", CreatedAt: t0.Add(time.Minute), ExpiresAt: t0.Add(6 * time.Minute)}
	if blocked, err := repo.Save(ctx, third, time.Minute); err != nil || blocked != nil {
		t.Fatalf("save after cooldown should replace: blocked=%+v err=%v", blocked, err)
	}
	got, err := repo.RecordAttempt(ctx, phone)
	if err != nil || got == nil || got.CodeHash != "h3" || got.Attempts != 1 {
		t.Fatalf("replaced otp should start fresh: %+v err=%v", got, err)
	}
}

func TestOTPKeyExpiresWithTTLButOutlivesCooldown(t *testing.T) {
	ctx := context.Background()
	client, prefix, fastForward := testRedis(t)
	if fastForward == nil {
		t.Skip("needs miniredis time travel")
	}
	repo := NewOTPRepository(client, prefix)
	t0 := time.UnixMilli(time.Now().UnixMilli())

	// TTL (10s) shorter than cooldown (1m): the key must live for the cooldown.
	otp := &model.OTP{Phone: phone, CreatedAt: t0, ExpiresAt: t0.Add(10 * time.Second)}
	if _, err := repo.Save(ctx, otp, time.Minute); err != nil {
		t.Fatal(err)
	}
	fastForward(30 * time.Second)
	if got, _ := repo.RecordAttempt(ctx, phone); got == nil {
		t.Fatal("key should still exist during the cooldown")
	}
	fastForward(31 * time.Second)
	if got, _ := repo.RecordAttempt(ctx, phone); got != nil {
		t.Fatalf("key should be gone after max(ttl, cooldown), got %+v", got)
	}
}

func TestOTPRecordAttemptIsAtomic(t *testing.T) {
	ctx := context.Background()
	client, prefix, _ := testRedis(t)
	repo := NewOTPRepository(client, prefix)
	now := time.Now()
	if _, err := repo.Save(ctx, &model.OTP{Phone: phone, CreatedAt: now, ExpiresAt: now.Add(time.Minute)}, 0); err != nil {
		t.Fatal(err)
	}

	const n = 50
	seen := make([]bool, n+1)
	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			otp, err := repo.RecordAttempt(ctx, phone)
			if err != nil || otp == nil || otp.Attempts < 1 || otp.Attempts > n {
				t.Errorf("record attempt: %+v err=%v", otp, err)
				return
			}
			mu.Lock()
			seen[otp.Attempts] = true
			mu.Unlock()
		}()
	}
	wg.Wait()
	for i := 1; i <= n; i++ {
		if !seen[i] {
			t.Fatalf("attempt number %d was never returned", i)
		}
	}
}

func TestOTPMarkUsedAndDelete(t *testing.T) {
	ctx := context.Background()
	client, prefix, _ := testRedis(t)
	repo := NewOTPRepository(client, prefix)
	now := time.Now()

	if got, err := repo.RecordAttempt(ctx, phone); err != nil || got != nil {
		t.Fatalf("missing otp: got %+v err=%v", got, err)
	}
	if ok, err := repo.MarkUsed(ctx, phone); err != nil || ok {
		t.Fatalf("MarkUsed on a missing otp: ok=%v err=%v", ok, err)
	}

	if _, err := repo.Save(ctx, &model.OTP{Phone: phone, CreatedAt: now, ExpiresAt: now.Add(time.Minute)}, 0); err != nil {
		t.Fatal(err)
	}
	if ok, err := repo.MarkUsed(ctx, phone); err != nil || !ok {
		t.Fatalf("first MarkUsed: ok=%v err=%v", ok, err)
	}
	if ok, _ := repo.MarkUsed(ctx, phone); ok {
		t.Fatal("second MarkUsed must fail")
	}
	if got, _ := repo.RecordAttempt(ctx, phone); got == nil || !got.Used {
		t.Fatalf("used flag not persisted: %+v", got)
	}

	if err := repo.Delete(ctx, phone); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.RecordAttempt(ctx, phone); got != nil {
		t.Fatal("otp should be gone after Delete")
	}
}

func TestHealthChecker(t *testing.T) {
	client, _, _ := testRedis(t)
	h := NewHealthChecker(client)
	if h.Name() != "redis" || h.Check(context.Background()) != nil {
		t.Fatal("healthy redis should pass the check")
	}
	_ = client.Close()
	if h.Check(context.Background()) == nil {
		t.Fatal("closed client should fail the check")
	}
}

func TestNewClientFailsFast(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := NewClient(ctx, "redis://127.0.0.1:1"); err == nil {
		t.Fatal("unreachable redis should fail at startup")
	}
	if _, err := NewClient(ctx, "not a url"); err == nil {
		t.Fatal("invalid url should fail")
	}
}
