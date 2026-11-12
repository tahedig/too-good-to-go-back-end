package dto

// PublicOfferResponse represents an available offer card under the store's public page.
type PublicOfferResponse struct {
	PublicID          string  `json:"id" example:"off-550e8400-e29b-41d4-a716-446655440000"`
	Title             string  `json:"title" example:"نان صبحگاهی"`
	CategoryName      string  `json:"category_name" example:"نانوایی"`
	DistanceText      string  `json:"distance_text,omitempty" example:"۰٫۶ کیلومتر"`
	ImageURL          *string `json:"image_url,omitempty" example:"https://example.com/bread.jpg"`
	OriginalPrice     int64   `json:"original_price" example:"115000"`
	Price             int64   `json:"price" example:"40000"`
	Currency          string  `json:"currency" example:"تومان"`
	QuantityAvailable int     `json:"quantity_available" example:"3"`
	AvailabilityBadge string  `json:"availability_badge" example:"۳ بسته موجود"`
	PickupWindow      string  `json:"pickup_window" example:"امروز، ۱۸:۰۰-۱۹:۰۰"`
}

// PublicWorkingScheduleItem represents a grouped schedule line in the store public view.
type PublicWorkingScheduleItem struct {
	Days     string `json:"days" example:"یکشنبه تا چهارشنبه"`
	Hours    string `json:"hours" example:"۰۸:۰۰-۱۹:۰۰"`
	IsClosed bool   `json:"is_closed" example:"false"`
}

// PublicStoreResponse represents the public store page data according to design.
type PublicStoreResponse struct {
	PublicID             string                      `json:"id" example:"store-550e8400-e29b-41d4-a716-446655440000"`
	Name                 string                      `json:"name" example:"نان صبحگاهی"`
	CategoryName         string                      `json:"category_name" example:"نانوایی"`
	City                 string                      `json:"city,omitempty" example:"تهران"`
	District             string                      `json:"district,omitempty" example:"ولیعصر"`
	HeaderSubtitle       string                      `json:"header_subtitle" example:"نانوایی · ولیعصر"`
	IsOpen               bool                        `json:"is_open" example:"true"`
	StatusBadge          string                      `json:"status_badge" example:"باز است"`
	Biography            string                      `json:"biography" example:"نانوایی محلی با نان‌های تازه و شیرینی‌های پخت روز."`
	PickupAddress        string                      `json:"pickup_address" example:"تهران، خیابان ولیعصر، پلاک ۱۲"`
	PickupInstructions   string                      `json:"pickup_instructions" example:"تحویل از پیشخوان نانوایی، ورودی خیابان ولیعصر."`
	PickupSummary        string                      `json:"pickup_summary,omitempty" example:"۳ موجود · تحویل امروز ۱۸:۰۰-۱۹:۰۰"`
	Location             StoreLocationResponse       `json:"location"`
	CoverImageURL        *string                     `json:"cover_image_url,omitempty" example:"https://example.com/cover.jpg"`
	WorkingHoursSchedule []PublicWorkingScheduleItem `json:"working_hours_schedule"`
	Offers               []PublicOfferResponse       `json:"offers"`
}
