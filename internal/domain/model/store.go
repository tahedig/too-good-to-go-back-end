package model

import "time"

// StoreLocation represents a store's geographic coordinates.
type StoreLocation struct {
	Latitude  float64
	Longitude float64
}

// WorkingHour represents opening and closing hours for a specific day of the week.
type WorkingHour struct {
	DayOfWeek int    // 0 = Saturday, 1 = Sunday, 2 = Monday, 3 = Tuesday, 4 = Wednesday, 5 = Thursday, 6 = Friday
	DayName   string // e.g. "saturday", "sunday", ...
	DayNameFa string // e.g. "شنبه", "یکشنبه", ...
	OpenTime  string // e.g. "08:00"
	CloseTime string // e.g. "19:00"
	IsClosed  bool   // true if تعطیل
}

// StoreImage represents a store photo in the store gallery.
type StoreImage struct {
	ID        string
	URL       string
	IsPrimary bool
	SortOrder int
}

// Store represents a vendor-owned store.
type Store struct {
	ID                  int64
	PublicID            string
	VendorID            int64
	VendorStatus        string
	CategoryID          *int16
	CategoryName        string
	OwnerName           string
	Name                string
	Slug                string
	Biography           string
	PickupInstructions  string
	Phone               *string
	City                string
	District            *string
	AddressLine         string
	Location            StoreLocation
	LogoURL             *string
	CoverImageURL       *string
	WorkingHours        []WorkingHour
	Images              []StoreImage
	ActiveProductsCount int
	IsActive            bool
	AcceptsOrders       bool
	RatingAvg           float64
	RatingCount         int
	TrustScore          float64
	MealsSaved          int
	CreatedAt           time.Time
	UpdatedAt           time.Time
	DeletedAt           *time.Time
}
