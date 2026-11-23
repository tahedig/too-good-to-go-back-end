package model

import (
	"strings"
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
)

// Reservation is a customer-owned reservation persisted in the orders and
// order_items tables (TEAM-49 & TEAM-51). IDs are public UUIDs; amounts are in Toman.
type Reservation struct {
	ID          string
	Reference   string
	PickupCode  string
	Status      enum.OrderStatus
	Store       ReservationStore
	Offer       ReservationOffer
	ItemsTotal  int64
	ServiceFee  int64
	TotalAmount int64
	Currency    string
	PickupStart *time.Time
	PickupEnd   *time.Time
	CreatedAt   time.Time
}

// ReservationStore is the store where the customer picks up the reservation.
type ReservationStore struct {
	ID                 string
	Name               string
	City               string
	AddressLine        string
	PickupInstructions string
	Latitude           *float64
	Longitude          *float64
}

// ReservationOffer is the snapshot of the reserved offer.
type ReservationOffer struct {
	ID                string
	Title             string
	OfferType         string
	ImageURL          *string
	CategoryName      string
	Quantity          int
	UnitPrice         int64
	UnitOriginalPrice int64
}

// ReservationReference derives a short, human-readable reservation reference
// from a reservation's public UUID (e.g. "THDG-7C9E6679").
func ReservationReference(publicID string) string {
	clean := strings.ReplaceAll(strings.TrimSpace(publicID), "-", "")
	if len(clean) >= 8 {
		return "THDG-" + strings.ToUpper(clean[:8])
	}
	if clean == "" {
		return ""
	}
	return "THDG-" + strings.ToUpper(clean)
}
