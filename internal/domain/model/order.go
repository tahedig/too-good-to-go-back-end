package model

import (
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
)

// Order is an order as its buyer sees it in their order history. IDs are
// public IDs (UUIDs); amounts are in Toman.
type Order struct {
	ID     string
	Status enum.OrderStatus
	// PaymentStatus is the status of the latest payment attempt, or "" when
	// payment has not been started.
	PaymentStatus string
	Store         OrderStore
	Items         []OrderItem
	ItemsTotal    int64
	ServiceFee    int64
	TotalAmount   int64
	// PickupStart and PickupEnd span the pickup windows of the ordered
	// offers. They are nil only for an order without items.
	PickupStart *time.Time
	PickupEnd   *time.Time
	// ReservedUntil is the payment deadline of a pending_payment order.
	ReservedUntil time.Time
	CreatedAt     time.Time
}

// OrderStore is the store an order was placed with. Its address is also the
// pickup address.
type OrderStore struct {
	ID          string
	Name        string
	City        string
	AddressLine string
}

// OrderItem is one ordered offer, with the title it had when ordered.
type OrderItem struct {
	Title     string
	OfferType string // surprise_bag or specific_item
	Quantity  int
	UnitPrice int64
}

// VendorOrderCustomer represents customer info in vendor reservations.
type VendorOrderCustomer struct {
	ID    string
	Name  string
	Phone string
}

// VendorOrderPackage represents package info in vendor reservations.
type VendorOrderPackage struct {
	ID       string
	Title    string
	ImageURL string
}

// VendorOrderTimeSlot represents pickup time boundaries.
type VendorOrderTimeSlot struct {
	StartTime string // e.g. "18:00"
	EndTime   string // e.g. "19:00"
}

// VendorOrder represents a reservation/order for the vendor panel.
type VendorOrder struct {
	ID             string
	NumericID      int64
	OrderNumber    string
	Customer       VendorOrderCustomer
	Package        VendorOrderPackage
	Quantity       int
	UnitPrice      int64
	TotalPrice     int64
	Status         enum.VendorOrderStatus
	DBStatus       enum.OrderStatus
	PickupDate     string // "YYYY-MM-DD"
	PickupTimeSlot VendorOrderTimeSlot
	PickupCode     string
	DeliveredAt    *time.Time
	CreatedAt      time.Time
}

// VendorOrderStatusUpdateResult is the domain model output of a vendor order status transition.
type VendorOrderStatusUpdateResult struct {
	Success   bool
	OrderID   string
	Status    enum.VendorOrderStatus
	UpdatedAt time.Time
	Message   string
}

// VendorOrderSummary represents daily aggregated summary stats for vendor orders.
type VendorOrderSummary struct {
	Date                    string
	TotalReservedPackages   int
	ActiveReservationsCount int
	DeliveredPackagesCount  int
	TotalRevenue            int64
}

// VerifyPickupResult is the domain model output of order pickup verification.
type VerifyPickupResult struct {
	Success     bool
	OrderID     string
	Status      enum.VendorOrderStatus
	DeliveredAt time.Time
	Message     string
}

// VendorStoreActive holds basic info of the vendor's active store.
type VendorStoreActive struct {
	ID       string
	Name     string
	Address  string
	Phone    string
	IsActive bool
}
