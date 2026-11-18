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
