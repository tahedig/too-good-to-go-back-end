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

type OrderService interface {
	// ListMyOrders lists the orders of the user with this public ID. Only
	// their own orders are ever returned.
	ListMyOrders(ctx context.Context, userID string, in ListOrdersInput) (*OrderPage, error)
}
