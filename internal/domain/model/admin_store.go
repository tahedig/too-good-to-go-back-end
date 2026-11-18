package model

import (
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
)

// AdminStoreDetail holds the full store profile shown on the admin store details page.
type AdminStoreDetail struct {
	ID           string
	Name         string
	OwnerName    string
	Category     string
	Phone        string
	Biography    string
	ImageURL     *string
	IsActive     bool
	VendorStatus enum.StoreStatus
	CreatedAt    time.Time
	Location     AdminStoreLocation
	WorkingHours []AdminStoreWorkingHour
}

type AdminStoreLocation struct {
	Latitude    float64
	Longitude   float64
	City        string
	District    *string
	AddressLine string
}

type AdminStoreWorkingHour struct {
	DayOfWeek int
	OpenTime  *string
	CloseTime *string
	IsClosed  bool
}

// AdminStoreSubscription holds the seller's current subscription and upgrade request.
type AdminStoreSubscription struct {
	ID             string
	StoreID        string
	Status         string
	CurrentPlan    string
	CommissionRate float64
	PeriodStart    time.Time
	PeriodEnd      time.Time
	RemainingDays  int
	UpgradeRequest *string
}

// AdminOffer holds product/surprise-bag details for the admin store & offer detail views.
type AdminOffer struct {
	ID                string
	StoreID           string
	StoreName         string
	OwnerName         string
	Category          string
	Title             string
	Description       string
	OfferType         string
	ImageURL          *string
	OriginalPrice     int64
	DiscountPrice     int64
	QuantityTotal     int
	QuantityAvailable int
	Status            string
	IsActive          bool
	PickupStart       time.Time
	PickupEnd         time.Time
	CreatedAt         time.Time
}
