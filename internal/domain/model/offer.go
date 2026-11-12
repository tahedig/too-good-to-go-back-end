package model

import "time"

// Offer represents a surprise bag or food offer listed by a store for public reservation.
type Offer struct {
	ID                int64
	PublicID          string
	StoreID           int64
	Title             string
	Description       string
	ImageURL          *string
	OriginalPrice     int64
	Price             int64
	Currency          string
	QuantityAvailable int
	PickupStart       time.Time
	PickupEnd         time.Time
	PickupWindowText  string
	DistanceKm        *float64
	IsActive          bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// IsAvailable returns true if the offer has stock and is active.
func (o *Offer) IsAvailable() bool {
	return o.IsActive && o.QuantityAvailable > 0
}
