package dto

import "time"

type AdminStoreDetailResponse struct {
	ID           string                     `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name         string                     `json:"name" example:"نان صبحگاهی"`
	OwnerName    string                     `json:"owner_name" example:"آیدا کریمی"`
	Category     string                     `json:"category" example:"نانوایی"`
	Phone        string                     `json:"phone" example:"09125550120"`
	Biography    string                     `json:"biography" example:"نانوایی سنگک سنتی"`
	ImageURL     *string                    `json:"image_url,omitempty" example:"https://example.com/store.jpg"`
	IsActive     bool                       `json:"is_active" example:"true"`
	VendorStatus string                     `json:"vendor_status" example:"approved"`
	CreatedAt    time.Time                  `json:"created_at" example:"2026-10-17T08:00:00Z"`
	Location     AdminStoreLocationResponse `json:"location"`
	WorkingHours []AdminStoreWorkingHourDTO `json:"working_hours"`
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

type AdminStoreSubscriptionResponse struct {
	ID             string    `json:"id" example:"660e8400-e29b-41d4-a716-446655440000"`
	StoreID        string    `json:"store_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Status         string    `json:"status" example:"pending_review"`
	CurrentPlan    string    `json:"current_plan" example:"پایه - رایگان"`
	CommissionRate float64   `json:"commission_rate" example:"10"`
	PeriodStart    time.Time `json:"period_start" example:"2026-09-23T00:00:00Z"`
	PeriodEnd      time.Time `json:"period_end" example:"2026-10-22T23:59:59Z"`
	RemainingDays  int       `json:"remaining_days" example:"3"`
	UpgradeRequest *string   `json:"upgrade_request,omitempty" example:"حرفه‌ای"`
}

type AdminOfferResponse struct {
	ID                string    `json:"id" example:"770e8400-e29b-41d4-a716-446655440000"`
	StoreID           string    `json:"store_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	StoreName         string    `json:"store_name" example:"نان صبحگاهی"`
	OwnerName         string    `json:"owner_name" example:"آیدا کریمی"`
	Category          string    `json:"category" example:"نانوایی"`
	Title             string    `json:"title" example:"بسته غافلگیری نانوایی"`
	Description       string    `json:"description" example:"ترکیبی از نان‌های تازه پایان روز"`
	OfferType         string    `json:"offer_type" example:"surprise_bag"`
	ImageURL          *string   `json:"image_url,omitempty" example:"https://example.com/offer.jpg"`
	OriginalPrice     int64     `json:"original_price" example:"115000"`
	DiscountPrice     int64     `json:"discount_price" example:"40000"`
	QuantityTotal     int       `json:"quantity_total" example:"5"`
	QuantityAvailable int       `json:"quantity_available" example:"3"`
	Status            string    `json:"status" example:"active"`
	IsActive          bool      `json:"is_active" example:"true"`
	PickupStart       time.Time `json:"pickup_start" example:"2026-10-18T15:00:00Z"`
	PickupEnd         time.Time `json:"pickup_end" example:"2026-10-18T19:00:00Z"`
	CreatedAt         time.Time `json:"created_at" example:"2026-10-18T08:00:00Z"`
}

type AdminOfferListResponse struct {
	Items []AdminOfferResponse `json:"items"`
}

type AdminStoreOrderResponse struct {
	ID           string    `json:"id" example:"880e8400-e29b-41d4-a716-446655440000"`
	StoreID      string    `json:"store_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	CustomerName string    `json:"customer_name" example:"لیلا علیزاده"`
	Quantity     int       `json:"quantity" example:"1"`
	TotalAmount  int64     `json:"total_amount" example:"40000"`
	Status       string    `json:"status" example:"paid"`
	CreatedAt    time.Time `json:"created_at" example:"2026-10-18T10:30:00Z"`
}

type AdminStoreOrderListResponse struct {
	Items    []AdminStoreOrderResponse `json:"items"`
	Total    int                       `json:"total" example:"3"`
	Page     int                       `json:"page" example:"1"`
	PageSize int                       `json:"page_size" example:"20"`
}
