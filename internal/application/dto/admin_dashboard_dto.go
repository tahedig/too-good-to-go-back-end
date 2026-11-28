package dto

import "time"

// AdminDashboardResponse is the response of GET /admin/dashboard.
// Monetary amounts are in Toman and timestamps are in UTC.
type AdminDashboardResponse struct {
	Totals         AdminDashboardTotalsResponse      `json:"totals"`
	Applications   AdminApplicationBreakdownResponse `json:"applications"`
	Reservations   AdminReservationBreakdownResponse `json:"reservations"`
	RecentActivity []AdminActivityItemResponse       `json:"recent_activity"`
}

// AdminDashboardTotalsResponse holds the platform KPI cards shown on the admin dashboard.
type AdminDashboardTotalsResponse struct {
	Customers             int   `json:"customers" example:"2"`
	Vendors               int   `json:"vendors" example:"4"`
	RegisteredStores      int   `json:"registered_stores" example:"5"`
	ActiveStores          int   `json:"active_stores" example:"1"`
	ActiveOffers          int   `json:"active_offers" example:"1"`
	PendingApplications   int   `json:"pending_applications" example:"3"`
	TotalReservations     int   `json:"total_reservations" example:"2"`
	ActiveReservations    int   `json:"active_reservations" example:"2"`
	CompletedReservations int   `json:"completed_reservations" example:"0"`
	PlatformCommission    int64 `json:"platform_commission" example:"200000"`
}

// AdminApplicationBreakdownResponse holds store application counts per status.
type AdminApplicationBreakdownResponse struct {
	Pending   int `json:"pending" example:"3"`
	Approved  int `json:"approved" example:"1"`
	Rejected  int `json:"rejected" example:"1"`
	Suspended int `json:"suspended" example:"0"`
}

// AdminReservationBreakdownResponse holds reservation counts per order status.
type AdminReservationBreakdownResponse struct {
	PendingPayment int `json:"pending_payment" example:"0"`
	Paid           int `json:"paid" example:"2"`
	Ready          int `json:"ready" example:"0"`
	Completed      int `json:"completed" example:"0"`
	Cancelled      int `json:"cancelled" example:"0"`
	Expired        int `json:"expired" example:"0"`
	NoShow         int `json:"no_show" example:"0"`
	Active         int `json:"active" example:"2"`
	Total          int `json:"total" example:"2"`
}

// AdminActivityItemResponse is one row in the admin dashboard recent-activity feed.
type AdminActivityItemResponse struct {
	// Type: store_application, store_decision, or reservation.
	Type       string    `json:"type" example:"store_application"`
	EntityID   string    `json:"entity_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Title      string    `json:"title" example:"نان صبحگاهی · شعبه شریعتی"`
	ActorName  string    `json:"actor_name" example:"آیدا کریمی"`
	Status     string    `json:"status" example:"pending"`
	Action     string    `json:"action,omitempty" example:"approve"`
	Amount     *int64    `json:"amount,omitempty" example:"40000"`
	OccurredAt time.Time `json:"occurred_at" example:"2026-10-19T07:10:00Z"`
}
