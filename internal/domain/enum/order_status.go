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

// VendorOrderStatus represents order status in the vendor panel.
type VendorOrderStatus string

const (
	VendorOrderStatusActive         VendorOrderStatus = "active"
	VendorOrderStatusReadyForPickup VendorOrderStatus = "ready_for_pickup"
	VendorOrderStatusDelivered      VendorOrderStatus = "delivered"
	VendorOrderStatusCancelled      VendorOrderStatus = "cancelled"
)

// ToVendorStatus maps a database OrderStatus to a VendorOrderStatus.
func (s OrderStatus) ToVendorStatus() VendorOrderStatus {
	switch s {
	case OrderStatusReady:
		return VendorOrderStatusReadyForPickup
	case OrderStatusCompleted:
		return VendorOrderStatusDelivered
	case OrderStatusCancelled, OrderStatusExpired, OrderStatusNoShow:
		return VendorOrderStatusCancelled
	case OrderStatusPaid, OrderStatusPendingPayment:
		fallthrough
	default:
		return VendorOrderStatusActive
	}
}

// VendorFilterToOrderStatuses maps a vendor filter status to the corresponding DB OrderStatus values.
// Returns nil if filter is empty, "all", or unrecognized (meaning match all).
func VendorFilterToOrderStatuses(filter string) []OrderStatus {
	switch filter {
	case "active":
		return []OrderStatus{OrderStatusPaid, OrderStatusPendingPayment}
	case "ready_for_pickup":
		return []OrderStatus{OrderStatusReady}
	case "delivered":
		return []OrderStatus{OrderStatusCompleted}
	case "cancelled":
		return []OrderStatus{OrderStatusCancelled, OrderStatusExpired, OrderStatusNoShow}
	default:
		return nil
	}
}

// ParseVendorTargetStatus converts a vendor-requested target status into the DB OrderStatus
// and corresponding VendorOrderStatus. Supports ready_for_pickup/ready, cancelled, and no_show.
func ParseVendorTargetStatus(raw string) (OrderStatus, VendorOrderStatus, bool) {
	switch raw {
	case "ready_for_pickup", "ready":
		return OrderStatusReady, VendorOrderStatusReadyForPickup, true
	case "cancelled":
		return OrderStatusCancelled, VendorOrderStatusCancelled, true
	case "no_show":
		return OrderStatusNoShow, VendorOrderStatusCancelled, true
	default:
		return "", "", false
	}
}

// CanVendorTransition reports whether a vendor is allowed to transition an order
// from `from` to `to` according to the order state machine.
func CanVendorTransition(from, to OrderStatus) bool {
	switch from {
	case OrderStatusPaid:
		return to == OrderStatusReady || to == OrderStatusCompleted || to == OrderStatusCancelled || to == OrderStatusNoShow
	case OrderStatusReady:
		return to == OrderStatusCompleted || to == OrderStatusCancelled || to == OrderStatusNoShow
	default:
		return false
	}
}

