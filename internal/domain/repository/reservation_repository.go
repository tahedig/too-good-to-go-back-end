package repository

import (
	"context"
	"errors"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
)

// Domain sentinel errors returned by ReservationRepository implementations.
// ErrUserNotFound is shared with UserRepository.
var (
	ErrOfferNotFound       = errors.New("offer not found")
	ErrOfferUnavailable    = errors.New("offer is not available for reservation")
	ErrInsufficientStock   = errors.New("requested quantity exceeds available stock")
	ErrExceedsMaxPerUser   = errors.New("requested quantity exceeds per-user limit")
	ErrIdempotencyConflict = errors.New("idempotency key already used with different request parameters")
	ErrReservationNotFound = errors.New("reservation not found")
)

// CreateReservationParams holds the validated parameters for atomic reservation creation.
type CreateReservationParams struct {
	UserPublicID           string
	OfferPublicID          string
	Quantity               int
	IdempotencyKey         string
	IdempotencyPayloadHash string
}

// CreateReservationResult wraps the persisted reservation and whether it was
// replayed from an earlier idempotent request.
type CreateReservationResult struct {
	Reservation *model.Reservation
	Replayed    bool
}

// ReservationRepository persists and retrieves customer reservations using the
// orders and order_items tables with atomic inventory updates.
type ReservationRepository interface {
	// Create atomically validates availability, decrements offer stock, and
	// persists the reservation. Repeated calls with the same (UserPublicID,
	// IdempotencyKey) and matching payload hash return the existing reservation
	// with Replayed=true.
	Create(ctx context.Context, params CreateReservationParams) (*CreateReservationResult, error)

	// GetByPublicIDAndUser returns the reservation only if it belongs to
	// userPublicID, or ErrReservationNotFound otherwise.
	GetByPublicIDAndUser(ctx context.Context, reservationPublicID, userPublicID string) (*model.Reservation, error)
}
