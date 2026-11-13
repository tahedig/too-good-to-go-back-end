package enum

// OrderStatus mirrors the order_status enum in the database.
type OrderStatus string

const (
	OrderStatusPendingPayment OrderStatus = "pending_payment"
	OrderStatusPaid           OrderStatus = "paid"
	OrderStatusReady          OrderStatus = "ready"
	OrderStatusCompleted      OrderStatus = "completed"
	OrderStatusCancelled      OrderStatus = "cancelled"
	OrderStatusExpired        OrderStatus = "expired"
	OrderStatusNoShow         OrderStatus = "no_show"
)

// OrderGroup splits a customer's orders the way the My Orders page does.
type OrderGroup string

const (
	// OrderGroupUpcoming holds orders that still have a pickup ahead,
	// including those waiting for payment.
	OrderGroupUpcoming OrderGroup = "upcoming"
	// OrderGroupPast holds orders that reached a final status.
	OrderGroupPast OrderGroup = "past"
)

var orderGroups = map[OrderGroup][]OrderStatus{
	OrderGroupUpcoming: {OrderStatusPendingPayment, OrderStatusPaid, OrderStatusReady},
	OrderGroupPast:     {OrderStatusCompleted, OrderStatusCancelled, OrderStatusExpired, OrderStatusNoShow},
}

func (s OrderStatus) Valid() bool {
	return s.Group() != ""
}

// Group returns the group the status belongs to, or "" for an unknown status.
func (s OrderStatus) Group() OrderGroup {
	for group, statuses := range orderGroups {
		for _, status := range statuses {
			if status == s {
				return group
			}
		}
	}
	return ""
}

// Statuses returns the statuses in the group, or nil for an unknown group.
func (g OrderGroup) Statuses() []OrderStatus {
	return append([]OrderStatus(nil), orderGroups[g]...)
}
