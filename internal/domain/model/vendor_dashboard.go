package model

import (
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
)

// VendorDashboard is the complete summary returned for an authenticated vendor,
// scoped strictly to stores owned by that vendor (or one selected owned store).
// Monetary amounts are in Toman and timestamps are in UTC.
type VendorDashboard struct {
	ActiveStore          *VendorDashboardStore
	Stores               []VendorDashboardStore
	PendingStoreRequests []VendorPendingStoreBanner
	KPIs                 VendorDashboardKPIs
	ReservationCounts    VendorReservationCounts
	Today                VendorTodayActivity
	Financial            VendorDashboardFinancial
	NearestReservation   *VendorNearestReservation
	RecentReservations   []VendorDashboardReservation
}

// VendorDashboardStore is a summary of one store owned by the vendor.
type VendorDashboardStore struct {
	ID            string
	Name          string
	City          string
	AddressLine   string
	Status        enum.StoreStatus
	AcceptsOrders bool
	CreatedAt     time.Time
}

// VendorPendingStoreBanner represents a pending store application banner shown
// on the vendor dashboard.
type VendorPendingStoreBanner struct {
	ID          string
	StoreName   string
	City        string
	AddressLine string
	Status      enum.StoreStatus
	CreatedAt   time.Time
}

// VendorDashboardKPIs holds the top-level dashboard metrics.
type VendorDashboardKPIs struct {
	ReceivedRevenue   int64
	ReservedPackages  int
	AvailablePackages int
	ActiveOffers      int
}

// VendorReservationCounts holds reservation counts broken down by order status
// plus active/total rollups.
type VendorReservationCounts struct {
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

// VendorTodayActivity holds today's (Asia/Tehran) reservation and revenue metrics.
type VendorTodayActivity struct {
	ReservationsCount int
	PackagesReserved  int
	CompletedCount    int
	Revenue           int64
}

// VendorDashboardFinancial holds the financial summary and active plan metadata
// displayed in the vendor dashboard's financial card.
type VendorDashboardFinancial struct {
	ReceivedPayout   int64
	NetPeriodRevenue int64
	PendingPayout    int64
	AvailableBalance int64
	Plan             VendorDashboardPlan
}

// VendorDashboardPlan describes the vendor's current commission plan window.
type VendorDashboardPlan struct {
	Name           string
	CommissionRate float64
	PeriodStart    time.Time
	PeriodEnd      time.Time
	RemainingDays  int
}

// VendorNearestReservation is the next upcoming active reservation highlighted
// on the vendor dashboard.
type VendorNearestReservation struct {
	ID                     string
	Status                 enum.OrderStatus
	PickupCode             string
	CustomerName           string
	StoreID                string
	StoreName              string
	OfferTitle             string
	OfferImageURL          *string
	Quantity               int
	TotalAmount            int64
	PickupStart            *time.Time
	PickupEnd              *time.Time
	OfferQuantityAvailable int
	OfferQuantityReserved  int
	OfferQuantityTotal     int
	CreatedAt              time.Time
}

// VendorDashboardReservation is one item in the vendor's recent reservations list.
type VendorDashboardReservation struct {
	ID            string
	Status        enum.OrderStatus
	PaymentStatus string
	PickupCode    string
	CustomerName  string
	StoreID       string
	StoreName     string
	Items         []OrderItem
	ItemsTotal    int64
	TotalAmount   int64
	VendorPayout  int64
	PickupStart   *time.Time
	PickupEnd     *time.Time
	CreatedAt     time.Time
}
