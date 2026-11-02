package model

import "time"

// StoreLocation represents a store's geographic coordinates.
type StoreLocation struct {
	Latitude  float64
	Longitude float64
}

// Store represents a vendor-owned store.
type Store struct {
	ID                 int64
	PublicID           string
	VendorID           int64
	Biography          string
	Location           StoreLocation
	RatingAvg          float64
	TrustScore         float64
	Name               string
	PickupAddress      string
	PickupInstructions string
	ApprovalStatus     string
	AvailabilityStatus string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
