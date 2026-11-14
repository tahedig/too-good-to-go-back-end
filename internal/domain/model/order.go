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
