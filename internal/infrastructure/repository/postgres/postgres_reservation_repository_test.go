package postgres

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

func TestPostgresReservationCheckoutConcurrencyAndOwnership(t *testing.T) {
	f := newOrderFixture(t)
	ctx := context.Background()

	// Make the fixture store approved and the offer active with future pickup.
	var adminID int64
	f.scan(`INSERT INTO users (phone) VALUES ('+989120000001') RETURNING id`, nil, &adminID)
	if _, err := f.pool.Exec(ctx, `
UPDATE stores SET status = 'approved', approved_by = $1, approved_at = now(), accepts_orders = true WHERE id = $2`,
		adminID, f.storeID); err != nil {
		t.Fatal(err)
	}
	futureStart := time.Now().UTC().Add(3 * time.Hour)
	var offerPublicID string
	f.scan(`
UPDATE offers
   SET status = 'active', allergens_declared = true,
       quantity_total = 3, quantity_available = 3, max_per_user = 2,
       pickup_start = $1, pickup_end = $2
 WHERE id = $3
RETURNING public_id::text`, []any{futureStart, futureStart.Add(time.Hour), f.offerID}, &offerPublicID)

	repo := NewReservationRepository(f.pool)
	alice := f.buyer("+989121111111")
	bob := f.buyer("+989122222222")

	// Create reservation with idempotency key
	res1, err := repo.Create(ctx, repository.CreateReservationParams{
		UserPublicID:           alice.publicID,
		OfferPublicID:          offerPublicID,
		Quantity:               1,
		IdempotencyKey:         "pg-idem-1",
		IdempotencyPayloadHash: "hash-1",
	})
	if err != nil {
		t.Fatalf("create reservation: %v", err)
	}
	if res1.Replayed || res1.Reservation.Status != enum.OrderStatusPaid || res1.Reservation.TotalAmount != 40000 {
		t.Fatalf("unexpected reservation: %+v", res1)
	}

	// Idempotent replay
	resReplay, err := repo.Create(ctx, repository.CreateReservationParams{
		UserPublicID:           alice.publicID,
		OfferPublicID:          offerPublicID,
		Quantity:               1,
		IdempotencyKey:         "pg-idem-1",
		IdempotencyPayloadHash: "hash-1",
	})
	if err != nil || !resReplay.Replayed || resReplay.Reservation.ID != res1.Reservation.ID {
		t.Fatalf("expected replay of %s, got res=%+v err=%v", res1.Reservation.ID, resReplay, err)
	}

	// Idempotency conflict with different payload hash
	_, err = repo.Create(ctx, repository.CreateReservationParams{
		UserPublicID:           alice.publicID,
		OfferPublicID:          offerPublicID,
		Quantity:               2,
		IdempotencyKey:         "pg-idem-1",
		IdempotencyPayloadHash: "hash-2",
	})
	if !errors.Is(err, repository.ErrIdempotencyConflict) {
		t.Fatalf("want ErrIdempotencyConflict, got %v", err)
	}

	// Ownership enforcement on GetByPublicIDAndUser
	got, err := repo.GetByPublicIDAndUser(ctx, res1.Reservation.ID, alice.publicID)
	if err != nil || got.ID != res1.Reservation.ID {
		t.Fatalf("owner fetch failed: got=%+v err=%v", got, err)
	}
	if _, err := repo.GetByPublicIDAndUser(ctx, res1.Reservation.ID, bob.publicID); !errors.Is(err, repository.ErrReservationNotFound) {
		t.Fatalf("non-owner fetch: want ErrReservationNotFound, got %v", err)
	}

	// Concurrent checkout for remaining 2 units across 6 goroutines -> exactly 2 succeed
	var (
		wg       sync.WaitGroup
		okCount  atomic.Int32
		errCount atomic.Int32
	)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repo.Create(ctx, repository.CreateReservationParams{
				UserPublicID:  bob.publicID,
				OfferPublicID: offerPublicID,
				Quantity:      1,
			})
			if err == nil {
				okCount.Add(1)
			} else if errors.Is(err, repository.ErrInsufficientStock) || errors.Is(err, repository.ErrOfferUnavailable) {
				errCount.Add(1)
			} else {
				t.Errorf("unexpected concurrent error: %v", err)
			}
		}()
	}
	wg.Wait()
	if okCount.Load() != 2 || errCount.Load() != 4 {
		t.Fatalf("concurrent checkout: ok=%d err=%d, want ok=2 err=4", okCount.Load(), errCount.Load())
	}
}
