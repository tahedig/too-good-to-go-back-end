package serviceinterface

import (
	"context"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
)

// CreateReservationInput is the service input for POST /reservations (TEAM-49).
type CreateReservationInput struct {
	OfferID        string
	Quantity       int
	TermsAccepted  *bool
	IdempotencyKey string
}

// CreateReservationOutput wraps the created (or replayed) reservation DTO.
type CreateReservationOutput struct {
	Reservation *dto.ReservationResponse
	Replayed    bool
}

// ReservationService handles customer reservation checkout (TEAM-49) and
// confirmation details (TEAM-51).
type ReservationService interface {
	CreateReservation(ctx context.Context, userPublicID string, in CreateReservationInput) (*CreateReservationOutput, error)
	GetReservation(ctx context.Context, userPublicID, reservationPublicID string) (*dto.ReservationResponse, error)
}
