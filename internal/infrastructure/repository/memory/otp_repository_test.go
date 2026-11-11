package memory

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
)

const phone = "+989123456789"

func TestSaveRespectsCooldown(t *testing.T) {
	ctx := context.Background()
	repo := NewOTPRepository()
	t0 := time.Unix(1_700_000_000, 0)

	save := func(hash string, at time.Time, cooldown time.Duration) *model.OTP {
		t.Helper()
		blocked, err := repo.Save(ctx, &model.OTP{Phone: phone, CodeHash: hash, CreatedAt: at}, cooldown)
		if err != nil {
			t.Fatal(err)
		}
		return blocked
	}

	if blocked := save("first", t0, time.Minute); blocked != nil {
		t.Fatalf("first save should not be blocked: %+v", blocked)
	}
	if blocked := save("second", t0.Add(30*time.Second), time.Minute); blocked == nil || blocked.CodeHash != "first" {
		t.Fatalf("save inside cooldown should be blocked by the first otp, got %+v", blocked)
	}
	if blocked := save("third", t0.Add(time.Minute), time.Minute); blocked != nil {
		t.Fatal("save after the cooldown should replace the otp")
	}
	if blocked := save("fourth", t0.Add(time.Minute), 0); blocked != nil {
		t.Fatal("zero cooldown should never block")
	}
}

// Parallel guesses must each see a distinct attempt number, otherwise the
// service's attempt limit could be bypassed.
func TestRecordAttemptIsAtomic(t *testing.T) {
	ctx := context.Background()
	repo := NewOTPRepository()
	if _, err := repo.Save(ctx, &model.OTP{Phone: phone}, 0); err != nil {
		t.Fatal(err)
	}

	const n = 100
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
			if err != nil || otp == nil {
				t.Error("record attempt failed")
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

func TestMarkUsedOnlyOnce(t *testing.T) {
	ctx := context.Background()
	repo := NewOTPRepository()
	if _, err := repo.Save(ctx, &model.OTP{Phone: phone}, 0); err != nil {
		t.Fatal(err)
	}

	if ok, _ := repo.MarkUsed(ctx, phone); !ok {
		t.Fatal("first MarkUsed should succeed")
	}
	if ok, _ := repo.MarkUsed(ctx, phone); ok {
		t.Fatal("second MarkUsed must fail")
	}
	if ok, _ := repo.MarkUsed(ctx, "+989120000000"); ok {
		t.Fatal("MarkUsed on a missing otp must fail")
	}
}
