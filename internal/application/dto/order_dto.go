package dto

import "time"

// OrderListResponse is one page of the signed-in user's orders, newest first.
type OrderListResponse struct {
	Orders []OrderResponse `json:"orders"`
	// NextCursor fetches the next page (?cursor=...); null on the last page.
	NextCursor *string `json:"next_cursor" example:"MjAyNi0xMS0xM1QxMDowMDowMFp8N2M5ZTY2NzktNzQyNS00MGRlLTk0NGItZTA3ZmMxZjkwYWU3"`
}

// OrderResponse is one order card. Amounts are in Toman; times are UTC.
type OrderResponse struct {
	ID string `json:"id" example:"7c9e6679-7425-40de-944b-e07fc1f90ae7"`
	// Status: pending_payment, paid, ready (upcoming) or completed,
	// cancelled, expired, no_show (past).
	Status string `json:"status" example:"paid"`
	// Group is "upcoming" or "past", matching the sections of the page.
	Group string `json:"group" example:"upcoming"`
	// PaymentStatus of the latest attempt: initiated, succeeded or failed;
	// null when payment has not been started.
	PaymentStatus *string             `json:"payment_status" example:"succeeded"`
	Store         OrderStoreResponse  `json:"store"`
	Items         []OrderItemResponse `json:"items"`
	ItemsTotal    int64               `json:"items_total" example:"40000"`
	ServiceFee    int64               `json:"service_fee" example:"0"`
	TotalAmount   int64               `json:"total_amount" example:"40000"`
	PickupStart   *time.Time          `json:"pickup_start" example:"2026-11-13T14:30:00Z"`
	PickupEnd     *time.Time          `json:"pickup_end" example:"2026-11-13T15:30:00Z"`
	// ReservedUntil is the payment deadline; set only for pending_payment.
	ReservedUntil *time.Time `json:"reserved_until,omitempty" example:"2026-11-13T10:10:00Z"`
	CreatedAt     time.Time  `json:"created_at" example:"2026-11-13T10:00:00Z"`
}

// OrderStoreResponse is the store an order is picked up from.
type OrderStoreResponse struct {
	ID            string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name          string `json:"name" example:"نان صبحگاهی"`
	PickupAddress string `json:"pickup_address" example:"تهران، خیابان ولیعصر، پلاک ۱۲"`
}

// OrderItemResponse is one ordered offer.
type OrderItemResponse struct {
	Title string `json:"title" example:"بسته شانسی نان"`
	// OfferType is surprise_bag or specific_item.
	OfferType string `json:"offer_type" example:"surprise_bag"`
	Quantity  int    `json:"quantity" example:"1"`
	UnitPrice int64  `json:"unit_price" example:"40000"`
}
