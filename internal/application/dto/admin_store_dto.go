package dto

import "time"

type AdminStoreDetailResponse struct {
	ID           string                        `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name         string                        `json:"name" example:"نان صبحگاهی"`
	OwnerName    string                        `json:"owner_name" example:"آیدا کریمی"`
	Category     string                        `json:"category" example:"نانوایی"`
	Phone        string                        `json:"phone" example:"09125550120"`
	Biography    string                        `json:"biography" example:"نانوایی سنگک سنتی"`
	ImageURL     *string                       `json:"image_url,omitempty" example:"https://example.com/store.jpg"`
	IsActive     bool                          `json:"is_active" example:"true"`
	VendorStatus string                        `json:"vendor_status" example:"approved"`
	CreatedAt    time.Time                     `json:"created_at" example:"2026-10-17T08:00:00Z"`
	Location     AdminStoreLocationResponse    `json:"location"`
	WorkingHours []AdminStoreWorkingHourDTO    `json:"working_hours"`
}

type AdminStoreLocationResponse struct {
	Latitude    float64 `json:"latitude" example:"35.7219"`
	Longitude   float64 `json:"longitude" example:"51.3347"`
	City        string  `json:"city" example:"تهران"`
	District    *string `json:"district,omitempty" example:"ولیعصر"`
	AddressLine string  `json:"address_line" example:"خیابان ولیعصر، پلاک ۱۲"`
}

type AdminStoreWorkingHourDTO struct {
	DayOfWeek int     `json:"day_of_week" example:"0"`
	OpenTime  *string `json:"open_time,omitempty" example:"08:00"`
	CloseTime *string `json:"close_time,omitempty" example:"19:00"`
	IsClosed  bool    `json:"is_closed" example:"false"`
}

type UpdateAdminStatusRequest struct {
	IsActive *bool `json:"is_active" example:"false"`
}
