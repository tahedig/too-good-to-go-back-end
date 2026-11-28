package dto

import "time"

// VendorDashboardResponse is the response of GET /vendor/dashboard.
// Monetary amounts are in Toman and timestamps are in UTC.
type VendorDashboardResponse struct {
	ActiveStore          *VendorDashboardStoreResponse       `json:"active_store"`
	Stores               []VendorDashboardStoreResponse      `json:"stores"`
	PendingStoreRequests []VendorPendingStoreBannerResponse  `json:"pending_store_requests"`
	KPIs                 VendorDashboardKPIsResponse         `json:"kpis"`
	ReservationCounts    VendorReservationCountsResponse     `json:"reservation_counts"`
	Today                VendorTodayActivityResponse         `json:"today"`
	Financial            VendorDashboardFinancialResponse    `json:"financial"`
	NearestReservation   *VendorNearestReservationResponse   `json:"nearest_reservation"`
	RecentReservations   []VendorDashboardReservationSummary `json:"recent_reservations"`
}

// VendorDashboardStoreResponse identifies one store owned by the vendor.
type VendorDashboardStoreResponse struct {
	ID            string    `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name          string    `json:"name" example:"نان صبحگاهی"`
	Address       string    `json:"address" example:"تهران، خیابان ولیعصر، پلاک ۱۲"`
	Status        string    `json:"status" example:"approved"`
	AcceptsOrders bool      `json:"accepts_orders" example:"true"`
	CreatedAt     time.Time `json:"created_at" example:"2026-10-15T08:00:00Z"`
}

// VendorPendingStoreBannerResponse represents a pending store application notice.
type VendorPendingStoreBannerResponse struct {
	ID        string    `json:"id" example:"550e8400-e29b-41d4-a716-446655440001"`
	StoreName string    `json:"store_name" example:"نان صبحگاهی · شعبه شریعتی"`
	Address   string    `json:"address" example:"تهران، خیابان شریعتی، پلاک ۲۴"`
	Status    string    `json:"status" example:"pending"`
	CreatedAt time.Time `json:"created_at" example:"2026-10-18T10:30:00Z"`
}

// VendorDashboardKPIsResponse holds the four primary KPI cards on the vendor dashboard.
type VendorDashboardKPIsResponse struct {
	ReceivedRevenue   int64 `json:"received_revenue" example:"1000000"`
	ReservedPackages  int   `json:"reserved_packages" example:"3"`
	AvailablePackages int   `json:"available_packages" example:"3"`
	ActiveOffers      int   `json:"active_offers" example:"1"`
}

// VendorReservationCountsResponse holds reservation counts by order status.
type VendorReservationCountsResponse struct {
	PendingPayment int `json:"pending_payment" example:"1"`
	Paid           int `json:"paid" example:"2"`
	Ready          int `json:"ready" example:"1"`
	Completed      int `json:"completed" example:"10"`
	Cancelled      int `json:"cancelled" example:"1"`
	Expired        int `json:"expired" example:"0"`
	NoShow         int `json:"no_show" example:"0"`
	Active         int `json:"active" example:"4"`
	Total          int `json:"total" example:"15"`
}

// VendorTodayActivityResponse summarizes today's (Asia/Tehran) vendor activity.
type VendorTodayActivityResponse struct {
	ReservationsCount int   `json:"reservations_count" example:"3"`
	PackagesReserved  int   `json:"packages_reserved" example:"4"`
	CompletedCount    int   `json:"completed_count" example:"1"`
	Revenue           int64 `json:"revenue" example:"120000"`
}

// VendorDashboardFinancialResponse holds the financial metrics and plan details.
type VendorDashboardFinancialResponse struct {
	ReceivedPayout   int64                       `json:"received_payout" example:"1000000"`
	NetPeriodRevenue int64                       `json:"net_period_revenue" example:"1800000"`
	PendingPayout    int64                       `json:"pending_payout" example:"500000"`
	AvailableBalance int64                       `json:"available_balance" example:"300000"`
	Plan             VendorDashboardPlanResponse `json:"plan"`
}

// VendorDashboardPlanResponse describes the vendor's active plan and commission rate.
type VendorDashboardPlanResponse struct {
	Name           string    `json:"name" example:"پایه · رایگان"`
	CommissionRate float64   `json:"commission_rate" example:"10"`
	PeriodStart    time.Time `json:"period_start" example:"2026-10-01T00:00:00Z"`
	PeriodEnd      time.Time `json:"period_end" example:"2026-11-01T00:00:00Z"`
	RemainingDays  int       `json:"remaining_days" example:"12"`
}

// VendorNearestReservationResponse highlights the next upcoming active reservation.
type VendorNearestReservationResponse struct {
	ID                     string     `json:"id" example:"7c9e6679-7425-40de-944b-e07fc1f90ae7"`
	Status                 string     `json:"status" example:"paid"`
	PickupCode             string     `json:"pickup_code" example:"4821"`
	CustomerName           string     `json:"customer_name" example:"لیلا علیزاده"`
	StoreID                string     `json:"store_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	StoreName              string     `json:"store_name" example:"نان صبحگاهی"`
	OfferTitle             string     `json:"offer_title" example:"بسته غافلگیری نانوایی"`
	OfferImageURL          *string    `json:"offer_image_url" example:"https://example.com/bag.jpg"`
	Quantity               int        `json:"quantity" example:"1"`
	TotalAmount            int64      `json:"total_amount" example:"40000"`
	PickupStart            *time.Time `json:"pickup_start" example:"2026-10-19T14:30:00Z"`
	PickupEnd              *time.Time `json:"pickup_end" example:"2026-10-19T15:30:00Z"`
	OfferQuantityAvailable int        `json:"offer_quantity_available" example:"3"`
	OfferQuantityReserved  int        `json:"offer_quantity_reserved" example:"3"`
	OfferQuantityTotal     int        `json:"offer_quantity_total" example:"6"`
	CreatedAt              time.Time  `json:"created_at" example:"2026-10-19T10:00:00Z"`
}

// VendorDashboardReservationSummary is one row in recent_reservations.
type VendorDashboardReservationSummary struct {
	ID            string              `json:"id" example:"7c9e6679-7425-40de-944b-e07fc1f90ae7"`
	Status        string              `json:"status" example:"paid"`
	PaymentStatus *string             `json:"payment_status" example:"succeeded"`
	PickupCode    string              `json:"pickup_code" example:"4821"`
	CustomerName  string              `json:"customer_name" example:"لیلا علیزاده"`
	StoreID       string              `json:"store_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	StoreName     string              `json:"store_name" example:"نان صبحگاهی"`
	Items         []OrderItemResponse `json:"items"`
	ItemsTotal    int64               `json:"items_total" example:"40000"`
	TotalAmount   int64               `json:"total_amount" example:"40000"`
	VendorPayout  int64               `json:"vendor_payout" example:"36000"`
	PickupStart   *time.Time          `json:"pickup_start" example:"2026-10-19T14:30:00Z"`
	PickupEnd     *time.Time          `json:"pickup_end" example:"2026-10-19T15:30:00Z"`
	CreatedAt     time.Time           `json:"created_at" example:"2026-10-19T10:00:00Z"`
}
