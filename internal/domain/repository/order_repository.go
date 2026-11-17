package repository

import (
	"context"
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
)

// OrderCursor marks the last order of a page. Orders are listed newest
// first, ordered by (CreatedAt, ID) descending.
type OrderCursor struct {
	CreatedAt time.Time
	ID        string
}

// OrderListQuery selects one page of a user's orders.
type OrderListQuery struct {
	UserID string // public ID of the buyer
	// Statuses limits the result to these statuses; nil means all. A non-nil
	// empty slice matches nothing.
	Statuses []enum.OrderStatus
	After    *OrderCursor // nil for the first page
	Limit    int
}

// VendorOrderFilter specifies query criteria for vendor reservations/orders.
type VendorOrderFilter struct {
	Date   string // "YYYY-MM-DD" or empty
	Status string // "all", "active", "ready_for_pickup", "delivered", "cancelled"
	Page   int    // >= 1
	Limit  int    // >= 1
}

type OrderRepository interface {
	// ListByUser returns up to q.Limit orders of q.UserID, newest first. It
	// never returns orders of other users.
	ListByUser(ctx context.Context, q OrderListQuery) ([]model.Order, error)

	// GetActiveStoreByVendorUser returns the active store belonging to the vendor user.
	GetActiveStoreByVendorUser(ctx context.Context, vendorUserID string) (*model.VendorStoreActive, error)

	// ListVendorOrders returns paginated orders for the vendor's active store and the total count.
	ListVendorOrders(ctx context.Context, vendorUserID string, filter VendorOrderFilter) ([]model.VendorOrder, int, error)

	// GetVendorOrderByID returns a single order belonging to the vendor's active store.
	GetVendorOrderByID(ctx context.Context, vendorUserID string, orderID string) (*model.VendorOrder, error)

	// GetVendorOrderSummary calculates summary statistics for orders on the given date.
	GetVendorOrderSummary(ctx context.Context, vendorUserID string, date string) (*model.VendorOrderSummary, error)

	// VerifyPickup marks an order as delivered after verifying ownership and optional pickup code.
	VerifyPickup(ctx context.Context, vendorUserID string, orderID string, pickupCode string) (*model.VerifyPickupResult, error)
}

