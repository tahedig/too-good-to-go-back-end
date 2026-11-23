package service

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

type fakeReservationRepo struct {
	createRes *repository.CreateReservationResult
	createErr error
	getRes    *model.Reservation
	getErr    error
	lastInput repository.CreateReservationParams
}

func (f *fakeReservationRepo) Create(_ context.Context, p repository.CreateReservationParams) (*repository.CreateReservationResult, error) {
	f.lastInput = p
	return f.createRes, f.createErr
}

func (f *fakeReservationRepo) GetByPublicIDAndUser(_ context.Context, _, _ string) (*model.Reservation, error) {
	return f.getRes, f.getErr
}

func TestReservationServiceValidationAndErrors(t *testing.T) {
	userID := "11111111-1111-4111-8111-111111111111"
	offerID := "22222222-2222-4222-8222-222222222222"
	resID := "33333333-3333-4333-8333-333333333333"
	falseVal := false

	repo := &fakeReservationRepo{}
	svc := NewReservationService(repo)

	// Missing auth -> 401
	if _, err := svc.CreateReservation(context.Background(), "", serviceinterface.CreateReservationInput{
		OfferID: offerID, Quantity: 1,
	}); err == nil {
		t.Fatal("expected 401")
	} else if appErr, ok := err.(exception.AppError); !ok || appErr.StatusCode() != http.StatusUnauthorized {
		t.Fatalf("want 401, got %v", err)
	}

	// Validation errors -> 400
	badInputs := []serviceinterface.CreateReservationInput{
		{OfferID: "bad-uuid", Quantity: 1},
		{OfferID: offerID, Quantity: 0},
		{OfferID: offerID, Quantity: 51},
		{OfferID: offerID, Quantity: 1, TermsAccepted: &falseVal},
		{OfferID: offerID, Quantity: 1, IdempotencyKey: strings.Repeat("a", 129)},
	}
	for i, in := range badInputs {
		_, err := svc.CreateReservation(context.Background(), userID, in)
		if appErr, ok := err.(exception.AppError); !ok || appErr.StatusCode() != http.StatusBadRequest {
			t.Errorf("case %d: want 400, got %v", i, err)
		}
	}

	// Repository error mappings
	errCases := []struct {
		repoErr error
		want    int
	}{
		{repository.ErrUserNotFound, http.StatusUnauthorized},
		{repository.ErrOfferNotFound, http.StatusNotFound},
		{repository.ErrOfferUnavailable, http.StatusConflict},
		{repository.ErrInsufficientStock, http.StatusConflict},
		{repository.ErrExceedsMaxPerUser, http.StatusBadRequest},
		{repository.ErrIdempotencyConflict, http.StatusConflict},
	}
	for _, tc := range errCases {
		repo.createErr = tc.repoErr
		_, err := svc.CreateReservation(context.Background(), userID, serviceinterface.CreateReservationInput{
			OfferID: offerID, Quantity: 1, IdempotencyKey: "k1",
		})
		if appErr, ok := err.(exception.AppError); !ok || appErr.StatusCode() != tc.want {
			t.Errorf("repoErr=%v: want %d, got %v", tc.repoErr, tc.want, err)
		}
	}

	// Happy path
	ps := time.Date(2026, 11, 23, 14, 30, 0, 0, time.UTC)
	pe := ps.Add(time.Hour)
	repo.createErr = nil
	repo.createRes = &repository.CreateReservationResult{
		Reservation: &model.Reservation{
			ID:          resID,
			PickupCode:  "4821",
			Status:      enum.OrderStatusPaid,
			Store:       model.ReservationStore{ID: "44444444-4444-4444-8444-444444444444", Name: "نان صبحگاهی", City: "تهران", AddressLine: "خیابان ولیعصر"},
			Offer:       model.ReservationOffer{ID: offerID, Title: "بسته غافلگیری", Quantity: 2, UnitPrice: 40000, UnitOriginalPrice: 115000},
			ItemsTotal:  80000,
			TotalAmount: 80000,
			PickupStart: &ps,
			PickupEnd:   &pe,
			CreatedAt:   ps.Add(-2 * time.Hour),
		},
	}
	out, err := svc.CreateReservation(context.Background(), userID, serviceinterface.CreateReservationInput{
		OfferID: offerID, Quantity: 2, IdempotencyKey: "req-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Replayed || out.Reservation.ID != resID || out.Reservation.Reference != "THDG-33333333" || out.Reservation.Status != "paid" {
		t.Fatalf("unexpected reservation output: %+v", out.Reservation)
	}
	if repo.lastInput.IdempotencyPayloadHash == "" {
		t.Error("expected non-empty idempotency payload hash")
	}

	// GetReservation: invalid UUID -> 400, not found -> 404, missing auth -> 401
	if _, err := svc.GetReservation(context.Background(), "", resID); err == nil {
		t.Fatal("expected 401")
	} else if appErr, ok := err.(exception.AppError); !ok || appErr.StatusCode() != http.StatusUnauthorized {
		t.Fatalf("want 401, got %v", err)
	}
	if _, err := svc.GetReservation(context.Background(), userID, "invalid"); err == nil {
		t.Fatal("expected 400")
	} else if appErr, ok := err.(exception.AppError); !ok || appErr.StatusCode() != http.StatusBadRequest {
		t.Fatalf("want 400, got %v", err)
	}
	repo.getErr = repository.ErrReservationNotFound
	if _, err := svc.GetReservation(context.Background(), userID, resID); err == nil {
		t.Fatal("expected 404")
	} else if appErr, ok := err.(exception.AppError); !ok || appErr.StatusCode() != http.StatusNotFound {
		t.Fatalf("want 404, got %v", err)
	}
}
