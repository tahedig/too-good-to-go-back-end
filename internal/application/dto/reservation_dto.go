package dto

import "time"

// CreateReservationRequest is the JSON payload for POST /reservations (TEAM-49).
type CreateReservationRequest struct {
	OfferID       string `json:"offer_id" example:"7d0c2a3e-1f5b-4a52-8c64-2f4e9b1a0c33"`
	Quantity      int    `json:"quantity" example:"1"`
	TermsAccepted *bool  `json:"terms_accepted,omitempty" example:"true"`
}

// ReservationStoreResponse is the pickup store summary on checkout/confirmation.
type ReservationStoreResponse struct {
	ID                 string                 `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name               string                 `json:"name" example:"نان صبحگاهی"`
	PickupAddress      string                 `json:"pickup_address" example:"تهران، خیابان ولیعصر، پلاک ۱۲"`
	PickupInstructions string                 `json:"pickup_instructions,omitempty" example:"از پیشخوان تحویل بگیرید."`
	Location           *StoreLocationResponse `json:"location,omitempty"`
}

// ReservationOfferResponse is the reserved offer snapshot shown on confirmation.
type ReservationOfferResponse struct {
	ID                string  `json:"id" example:"7d0c2a3e-1f5b-4a52-8c64-2f4e9b1a0c33"`
	Title             string  `json:"title" example:"بسته غافلگیری نانوایی"`
	OfferType         string  `json:"offer_type,omitempty" example:"surprise_bag"`
	ImageURL          *string `json:"image_url,omitempty"`
	CategoryName      string  `json:"category_name,omitempty" example:"نانوایی"`
	Quantity          int     `json:"quantity" example:"1"`
	UnitPrice         int64   `json:"unit_price" example:"40000"`
	UnitOriginalPrice int64   `json:"unit_original_price" example:"115000"`
}

// ReservationResponse is returned by POST /reservations (TEAM-49) and
// GET /reservations/{id} (TEAM-51). Amounts are in Toman; times are UTC.
type ReservationResponse struct {
	ID                   string                   `json:"id" example:"7c9e6679-7425-40de-944b-e07fc1f90ae7"`
	Reference            string                   `json:"reference" example:"THDG-7C9E6679"`
	PickupCode           string                   `json:"pickup_code" example:"4821"`
	Status               string                   `json:"status" example:"paid"`
	StatusLabel          string                   `json:"status_label" example:"رزرو تأیید شده"`
	Group                string                   `json:"group" example:"upcoming"`
	Store                ReservationStoreResponse `json:"store"`
	Offer                ReservationOfferResponse `json:"offer"`
	ItemsTotal           int64                    `json:"items_total" example:"40000"`
	ServiceFee           int64                    `json:"service_fee" example:"0"`
	TotalAmount          int64                    `json:"total_amount" example:"40000"`
	Currency             string                   `json:"currency" example:"تومان"`
	PickupStart          *time.Time               `json:"pickup_start,omitempty" example:"2026-11-13T14:30:00Z"`
	PickupEnd            *time.Time               `json:"pickup_end,omitempty" example:"2026-11-13T15:30:00Z"`
	PickupWindow         string                   `json:"pickup_window,omitempty" example:"امروز، ۱۸:۰۰-۱۹:۰۰"`
	CancellationDeadline *time.Time               `json:"cancellation_deadline,omitempty" example:"2026-11-13T12:30:00Z"`
	CreatedAt            time.Time                `json:"created_at" example:"2026-11-13T10:00:00Z"`
}
