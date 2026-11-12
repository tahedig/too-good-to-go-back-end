package dto

// WorkingHourDTO represents opening and closing hours for a day.
type WorkingHourDTO struct {
	DayOfWeek int    `json:"day_of_week" example:"0"` // 0 = Saturday, 1 = Sunday, ..., 6 = Friday
	DayName   string `json:"day_name" example:"saturday"`
	DayNameFa string `json:"day_name_fa" example:"شنبه"`
	OpenTime  string `json:"open_time" example:"08:00"`
	CloseTime string `json:"close_time" example:"19:00"`
	IsClosed  bool   `json:"is_closed" example:"false"`
}

// StoreImageDTO represents a store photo.
type StoreImageDTO struct {
	ID        string `json:"id" example:"img-1"`
	URL       string `json:"url" example:"https://example.com/store.jpg"`
	IsPrimary bool   `json:"is_primary" example:"true"`
	SortOrder int    `json:"sort_order" example:"1"`
}

// UpdateStoreRequest contains fields editable by the vendor for their store profile.
type UpdateStoreRequest struct {
	Name               string                 `json:"name" example:"نان صبحگاهی"`
	CategoryID         *int16                 `json:"category_id,omitempty" example:"1"`
	Phone              *string                `json:"phone,omitempty" example:"09124550120"`
	PickupAddress      string                 `json:"pickup_address" example:"تهران، خیابان ولیعصر، پلاک ۱۲"`
	Biography          string                 `json:"biography" example:"نانوایی محلی با نان‌های تازه و شیرینی‌های پخت روز."`
	PickupInstructions string                 `json:"pickup_instructions" example:"تحویل از پیشخوان نانوایی، ورودی خیابان ولیعصر."`
	Location           *StoreLocationResponse `json:"location,omitempty"`
	WorkingHours       []WorkingHourDTO       `json:"working_hours,omitempty"`
	Images             []StoreImageDTO        `json:"images,omitempty"`
}

// StoreLocationResponse represents the geographic coordinates of a store.
type StoreLocationResponse struct {
	Latitude  float64 `json:"latitude" example:"35.6892"`
	Longitude float64 `json:"longitude" example:"51.3890"`
}

// StoreResponse represents the full store profile data.
type StoreResponse struct {
	PublicID            string                `json:"public_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name                string                `json:"name" example:"نان صبحگاهی"`
	CategoryID          *int16                `json:"category_id,omitempty" example:"1"`
	CategoryName        string                `json:"category_name,omitempty" example:"نانوایی"`
	OwnerName           string                `json:"owner_name,omitempty" example:"آیدا کریمی"`
	Phone               *string               `json:"phone,omitempty" example:"09124550120"`
	Biography           string                `json:"biography" example:"نانوایی محلی با نان‌های تازه و شیرینی‌های پخت روز."`
	PickupAddress       string                `json:"pickup_address" example:"تهران، خیابان ولیعصر، پلاک ۱۲"`
	PickupInstructions  string                `json:"pickup_instructions,omitempty" example:"تحویل از پیشخوان نانوایی، ورودی خیابان ولیعصر."`
	Location            StoreLocationResponse `json:"location"`
	WorkingHours        []WorkingHourDTO      `json:"working_hours,omitempty"`
	WorkingHoursSummary string                `json:"working_hours_summary,omitempty" example:"شنبه تا چهارشنبه · ۰۸:۰۰ تا ۱۹:۰۰"`
	ClosedDaysSummary   string                `json:"closed_days_summary,omitempty" example:"پنج‌شنبه و جمعه تعطیل"`
	Images              []StoreImageDTO       `json:"images,omitempty"`
	CoverImageURL       *string               `json:"cover_image_url,omitempty"`
	ActiveProductsCount int                   `json:"active_products_count" example:"1"`
	RatingAvg           float64               `json:"rating_avg"`
	TrustScore          float64               `json:"trust_score"`
	ApprovalStatus      string                `json:"approval_status"`
	AvailabilityStatus  string                `json:"availability_status"`
	CreatedAt           string                `json:"created_at"`
	UpdatedAt           string                `json:"updated_at"`
}

// StoreListResponse represents the list of vendor stores.
type StoreListResponse struct {
	Stores []StoreResponse `json:"stores"`
}
