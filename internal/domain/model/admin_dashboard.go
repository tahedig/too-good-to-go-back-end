package model

import "time"

// AdminActivityType classifies entries in the admin dashboard recent-activity feed.
type AdminActivityType string

const (
	AdminActivityStoreApplication AdminActivityType = "store_application"
	AdminActivityStoreDecision    AdminActivityType = "store_decision"
	AdminActivityReservation      AdminActivityType = "reservation"
)

// Valid reports whether t is a supported admin activity type.
func (t AdminActivityType) Valid() bool {
	switch t {
	case AdminActivityStoreApplication, AdminActivityStoreDecision, AdminActivityReservation:
		return true
	}
	return false
}

// AdminDashboard is the platform-wide summary returned to admins.
// Monetary amounts are in Toman and timestamps are in UTC.
type AdminDashboard struct {
	Totals         AdminDashboardTotals
	Applications   AdminApplicationBreakdown
	Reservations   AdminReservationBreakdown
	RecentActivity []AdminActivityItem
}

// AdminDashboardTotals holds top-level platform counts and commission revenue.
type AdminDashboardTotals struct {
	Customers             int
	Vendors               int
	RegisteredStores      int
	ActiveStores          int
	ActiveOffers          int
	PendingApplications   int
	TotalReservations     int
	ActiveReservations    int
	CompletedReservations int
	PlatformCommission    int64
}

// AdminApplicationBreakdown holds store application counts per store_status.
type AdminApplicationBreakdown struct {
	Pending   int
	Approved  int
	Rejected  int
	Suspended int
}

// AdminReservationBreakdown holds reservation counts per order_status plus
// active and total rollups.
type AdminReservationBreakdown struct {
	PendingPayment int
	Paid           int
	Ready          int
	Completed      int
	Cancelled      int
	Expired        int
	NoShow         int
	Active         int
	Total          int
}

// AdminActivityItem is one entry in the concise admin recent-activity feed.
type AdminActivityItem struct {
	Type       AdminActivityType
	EntityID   string
	Title      string
	ActorName  string
	Status     string
	Action     string
	Amount     *int64
	OccurredAt time.Time
}
