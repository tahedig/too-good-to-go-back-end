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

type OrderRepository interface {
	// ListByUser returns up to q.Limit orders of q.UserID, newest first. It
	// never returns orders of other users.
	ListByUser(ctx context.Context, q OrderListQuery) ([]model.Order, error)
}
