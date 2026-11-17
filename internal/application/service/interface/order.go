package serviceinterface

import (
	"context"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
)

// ListOrdersInput is the raw query of a My Orders request; the service
// validates it.
type ListOrdersInput struct {
	Group  string // "", "all", "upcoming" or "past"
	Status string // comma-separated order statuses, e.g. "completed,cancelled"
	Limit  string // page size; empty for the default
	Cursor string // NextCursor of the previous page; empty for the first page
}

// OrderPage is one page of orders, newest first. NextCursor is empty on the
// last page.
type OrderPage struct {
	Orders     []model.Order
	NextCursor string
}

// VendorOrderListInput holds query filters for listing vendor orders.
type VendorOrderListInput struct {
	Date   string
	Status string
	Page   int
	Limit  int
}

// VendorOrderListPage wraps paginated vendor orders with pagination details.
type VendorOrderListPage struct {
	Orders     []model.VendorOrder
	TotalItems int
	Page       int
	Limit      int
	TotalPages int
}

type OrderService interface {
	// ListMyOrders lists the orders of the user with this public ID. Only
	// their own orders are ever returned.
	ListMyOrders(ctx context.Context, userID string, in ListOrdersInput) (*OrderPage, error)

	// GetVendorStore returns the active store belonging to the authenticated vendor.
	GetVendorStore(ctx context.Context, vendorUserID string) (*model.VendorStoreActive, error)

	// ListVendorOrders returns paginated vendor orders matching the filters.
	ListVendorOrders(ctx context.Context, vendorUserID string, in VendorOrderListInput) (*VendorOrderListPage, error)

	// GetVendorOrder returns details of a single vendor order.
	GetVendorOrder(ctx context.Context, vendorUserID string, orderID string) (*model.VendorOrder, error)

	// GetVendorOrderSummary returns daily summary stats for the vendor orders.
	GetVendorOrderSummary(ctx context.Context, vendorUserID string, date string) (*model.VendorOrderSummary, error)

	// VerifyPickup verifies the pickup code and marks the order as delivered.
	VerifyPickup(ctx context.Context, vendorUserID string, orderID string, pickupCode string) (*model.VerifyPickupResult, error)

	// UpdateVendorOrderStatus transitions an order to ready_for_pickup, cancelled, or no_show.
	UpdateVendorOrderStatus(ctx context.Context, vendorUserID string, orderID string, status string, reason string) (*model.VendorOrderStatusUpdateResult, error)
}

