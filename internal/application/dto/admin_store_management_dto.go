package dto

import "time"

type AdminManagedStoreResponse struct {
	ID                  string                     `json:"id"`
	Name                string                     `json:"name"`
	OwnerName           string                     `json:"owner_name"`
	Category            string                     `json:"category"`
	CategoryID          *int16                     `json:"category_id,omitempty"`
	CategoryName        string                     `json:"category_name"`
	Phone               string                     `json:"phone"`
	Biography           string                     `json:"biography"`
	PickupInstructions  string                     `json:"pickup_instructions"`
	City                string                     `json:"city"`
	District            *string                    `json:"district,omitempty"`
	AddressLine         string                     `json:"address_line"`
	Location            AdminStoreLocationResponse `json:"location"`
	ImageURL            *string                    `json:"image_url,omitempty"`
	LogoURL             *string                    `json:"logo_url,omitempty"`
	CoverImageURL       *string                    `json:"cover_image_url,omitempty"`
	Status              string                     `json:"status"`
	IsActive            bool                       `json:"is_active"`
	VendorStatus        string                     `json:"vendor_status"`
	ApprovalStatus      string                     `json:"approval_status"`
	AcceptsOrders       bool                       `json:"accepts_orders"`
	ActiveProductsCount int                        `json:"active_products_count"`
	RatingAvg           float64                    `json:"rating_avg"`
	RatingCount         int                        `json:"rating_count"`
	TrustScore          float64                    `json:"trust_score"`
	MealsSaved          int                        `json:"meals_saved"`
	WorkingHours        []AdminStoreWorkingHourDTO `json:"working_hours"`
	Images              []StoreImageDTO            `json:"images"`
	CreatedAt           time.Time                  `json:"created_at"`
	UpdatedAt           time.Time                  `json:"updated_at"`
}

type AdminManagedStoreListResponse struct {
	Items      []AdminManagedStoreResponse `json:"items"`
	Total      int                         `json:"total"`
	HasMore    bool                        `json:"has_more"`
	NextCursor *string                     `json:"next_cursor"`
}

// AdminManagedStoreUpdateRequest only binds editable profile fields.
// Protected fields in the request body are ignored by JSON decoding.
type AdminManagedStoreUpdateRequest struct {
	Name               *string                `json:"name"`
	Biography          *string                `json:"biography"`
	PickupInstructions *string                `json:"pickup_instructions"`
	Phone              *string                `json:"phone"`
	City               *string                `json:"city"`
	District           *string                `json:"district"`
	AddressLine        *string                `json:"address_line"`
	CategoryID         *int16                 `json:"category_id"`
	Location           *StoreLocationResponse `json:"location"`
	LogoURL            *string                `json:"logo_url"`
	CoverImageURL      *string                `json:"cover_image_url"`
	WorkingHours       *[]WorkingHourDTO      `json:"working_hours"`
	Images             *[]StoreImageDTO       `json:"images"`
	AcceptsOrders      *bool                  `json:"accepts_orders"`
}
