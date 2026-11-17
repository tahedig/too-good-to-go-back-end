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

// VendorStoreActiveResponse is the response for the vendor's active store details.
type VendorStoreActiveResponse struct {
	ID       string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name     string `json:"name" example:"نانوایی سنتی شمیران"`
	Address  string `json:"address" example:"تهران، خیابان ولیعصر، پلاک ۱۲"`
	Phone    string `json:"phone" example:"+982122334455"`
	IsActive bool   `json:"is_active" example:"true"`
}

// VendorCustomerDTO is customer info in vendor order representations.
type VendorCustomerDTO struct {
	ID    string `json:"id" example:"cust_101"`
	Name  string `json:"name" example:"لیلا علیزاده"`
	Phone string `json:"phone" example:"+989123456789"`
}

// VendorPackageDTO is package info in vendor order representations.
type VendorPackageDTO struct {
	ID       string `json:"id" example:"pkg_501"`
	Title    string `json:"title" example:"بستهٔ غافلگیری نانوایی"`
	ImageURL string `json:"image_url" example:"https://example.com/bread.jpg"`
}

// VendorTimeSlotDTO is the pickup time slot.
type VendorTimeSlotDTO struct {
	StartTime string `json:"start_time" example:"18:00"`
	EndTime   string `json:"end_time" example:"19:00"`
}

// VendorOrderItemResponse is one order item in the vendor orders list or detail.
type VendorOrderItemResponse struct {
	ID             string            `json:"id" example:"ord_12345"`
	OrderNumber    string            `json:"order_number" example:"TG-9821"`
	Customer       VendorCustomerDTO `json:"customer"`
	Package        VendorPackageDTO  `json:"package"`
	Quantity       int               `json:"quantity" example:"1"`
	UnitPrice      int64             `json:"unit_price" example:"40000"`
	TotalPrice     int64             `json:"total_price" example:"40000"`
	Status         string            `json:"status" example:"active"`
	PickupDate     string            `json:"pickup_date" example:"2026-10-19"`
	PickupTimeSlot VendorTimeSlotDTO `json:"pickup_time_slot"`
	CreatedAt      string            `json:"created_at" example:"2026-10-19T14:30:00Z"`
}

// VendorPaginationDTO contains pagination metadata for vendor orders list.
type VendorPaginationDTO struct {
	Page       int `json:"page" example:"1"`
	Limit      int `json:"limit" example:"20"`
	TotalItems int `json:"total_items" example:"2"`
	TotalPages int `json:"total_pages" example:"1"`
}

// VendorOrderListResponse is the paginated response for vendor orders list.
type VendorOrderListResponse struct {
	Orders     []VendorOrderItemResponse `json:"orders"`
	Pagination VendorPaginationDTO       `json:"pagination"`
}

// VendorOrderSummaryResponse represents daily summary statistics for vendor orders.
type VendorOrderSummaryResponse struct {
	Date                    string `json:"date" example:"2026-10-19"`
	TotalReservedPackages   int    `json:"total_reserved_packages" example:"3"`
	ActiveReservationsCount int    `json:"active_reservations_count" example:"2"`
	DeliveredPackagesCount  int    `json:"delivered_packages_count" example:"0"`
	TotalRevenue            int64  `json:"total_revenue" example:"120000"`
}

// VerifyPickupRequest is the request payload to verify an order pickup.
type VerifyPickupRequest struct {
	PickupCode string `json:"pickup_code" example:"1234"`
}

// VerifyPickupResponse is the response after successfully verifying an order pickup.
type VerifyPickupResponse struct {
	Success     bool   `json:"success" example:"true"`
	OrderID     string `json:"order_id" example:"ord_12345"`
	Status      string `json:"status" example:"delivered"`
	DeliveredAt string `json:"delivered_at" example:"2026-10-19T18:15:00Z"`
	Message     string `json:"message" example:"تحویل بسته با موفقیت ثبت شد"`
}

// UpdateVendorOrderStatusRequest is the request payload to transition a vendor order status.
type UpdateVendorOrderStatusRequest struct {
	Status string `json:"status" binding:"required" example:"ready_for_pickup" enums:"ready_for_pickup,ready,cancelled,no_show"`
	Reason string `json:"reason,omitempty" example:"عدم مراجعه مشتری"`
}

// UpdateVendorOrderStatusResponse is the response after transitioning a vendor order status.
type UpdateVendorOrderStatusResponse struct {
	Success   bool   `json:"success" example:"true"`
	OrderID   string `json:"order_id" example:"ord_12345"`
	Status    string `json:"status" example:"ready_for_pickup"`
	UpdatedAt string `json:"updated_at" example:"2026-10-19T18:00:00Z"`
	Message   string `json:"message" example:"وضعیت سفارش با موفقیت به‌روزرسانی شد"`
}

