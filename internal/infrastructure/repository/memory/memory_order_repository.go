package memory

import (
	"context"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

// OrderRepository keeps orders in memory. Nothing creates orders yet, so in
// development it starts empty; tests fill it with Add.
type OrderRepository struct {
	mu           sync.Mutex
	byUser       map[string][]model.Order
	vendorStores map[string]model.VendorStoreActive
	vendorOrders map[string][]model.VendorOrder
	offerStock   map[string]int
}

func NewOrderRepository() *OrderRepository {
	return &OrderRepository{
		byUser:       make(map[string][]model.Order),
		vendorStores: make(map[string]model.VendorStoreActive),
		vendorOrders: make(map[string][]model.VendorOrder),
		offerStock:   make(map[string]int),
	}
}

var _ repository.OrderRepository = (*OrderRepository)(nil)

// SetOfferStock sets available stock for an offer in memory (for testing inventory consistency).
func (r *OrderRepository) SetOfferStock(offerID string, qty int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.offerStock[offerID] = qty
}

// GetOfferStock returns available stock for an offer in memory.
func (r *OrderRepository) GetOfferStock(offerID string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.offerStock[offerID]
}

// AddVendorStore sets an active store for the vendor user.
func (r *OrderRepository) AddVendorStore(vendorUserID string, s model.VendorStoreActive) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.vendorStores[vendorUserID] = s
}

// AddVendorOrder adds a vendor order for testing.
func (r *OrderRepository) AddVendorOrder(vendorUserID string, o model.VendorOrder) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.vendorOrders[vendorUserID] = append(r.vendorOrders[vendorUserID], o)
}

// Add stores an order for the buyer with this public user ID.
func (r *OrderRepository) Add(userID string, o model.Order) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byUser[userID] = append(r.byUser[userID], o)
}

func (r *OrderRepository) ListByUser(_ context.Context, q repository.OrderListQuery) ([]model.Order, error) {
	r.mu.Lock()
	all := slices.Clone(r.byUser[q.UserID])
	r.mu.Unlock()

	// Same order as PostgreSQL: (created_at, id) descending.
	sort.Slice(all, func(i, j int) bool {
		return newerThan(all[i].CreatedAt, all[i].ID, all[j].CreatedAt, all[j].ID)
	})

	out := []model.Order{}
	for _, o := range all {
		if len(out) >= q.Limit {
			break
		}
		if q.Statuses != nil && !slices.Contains(q.Statuses, o.Status) {
			continue
		}
		if q.After != nil && !newerThan(q.After.CreatedAt, q.After.ID, o.CreatedAt, o.ID) {
			continue
		}
		o.Items = slices.Clone(o.Items)
		out = append(out, o)
	}
	return out, nil
}

// newerThan reports whether (t1, id1) comes before (t2, id2) in a newest-first
// listing. IDs are lowercase UUIDs, so string order matches PostgreSQL's.
func newerThan(t1 time.Time, id1 string, t2 time.Time, id2 string) bool {
	if !t1.Equal(t2) {
		return t1.After(t2)
	}
	return id1 > id2
}

func (r *OrderRepository) GetActiveStoreByVendorUser(_ context.Context, vendorUserID string) (*model.VendorStoreActive, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.vendorStores[vendorUserID]
	if !ok {
		return nil, exception.ErrStoreNotFound
	}
	return &s, nil
}

func (r *OrderRepository) ListVendorOrders(_ context.Context, vendorUserID string, filter repository.VendorOrderFilter) ([]model.VendorOrder, int, error) {
	r.mu.Lock()
	all := slices.Clone(r.vendorOrders[vendorUserID])
	r.mu.Unlock()

	// Filter orders
	filtered := make([]model.VendorOrder, 0, len(all))
	for _, o := range all {
		if filter.Date != "" && o.PickupDate != filter.Date {
			continue
		}
		if filter.Status != "" && filter.Status != "all" && string(o.Status) != filter.Status {
			continue
		}
		filtered = append(filtered, o)
	}

	// Sort newest first
	sort.Slice(filtered, func(i, j int) bool {
		return newerThan(filtered[i].CreatedAt, filtered[i].ID, filtered[j].CreatedAt, filtered[j].ID)
	})

	total := len(filtered)
	start := (filter.Page - 1) * filter.Limit
	if start >= total {
		return []model.VendorOrder{}, total, nil
	}
	end := start + filter.Limit
	if end > total {
		end = total
	}

	return slices.Clone(filtered[start:end]), total, nil
}

func (r *OrderRepository) GetVendorOrderByID(_ context.Context, vendorUserID string, orderID string) (*model.VendorOrder, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	orders := r.vendorOrders[vendorUserID]
	for _, o := range orders {
		if o.ID == orderID || strings.EqualFold(o.ID, orderID) {
			cp := o
			return &cp, nil
		}
	}
	return nil, exception.ErrOrderNotFound
}

func (r *OrderRepository) GetVendorOrderSummary(_ context.Context, vendorUserID string, date string) (*model.VendorOrderSummary, error) {
	r.mu.Lock()
	orders := slices.Clone(r.vendorOrders[vendorUserID])
	r.mu.Unlock()

	summary := &model.VendorOrderSummary{
		Date: date,
	}

	for _, o := range orders {
		if o.PickupDate != date {
			continue
		}
		switch o.Status {
		case enum.VendorOrderStatusActive:
			summary.ActiveReservationsCount++
			summary.TotalReservedPackages += o.Quantity
			summary.TotalRevenue += o.TotalPrice
		case enum.VendorOrderStatusReadyForPickup:
			summary.ActiveReservationsCount++
			summary.TotalReservedPackages += o.Quantity
			summary.TotalRevenue += o.TotalPrice
		case enum.VendorOrderStatusDelivered:
			summary.DeliveredPackagesCount++
			summary.TotalReservedPackages += o.Quantity
			summary.TotalRevenue += o.TotalPrice
		}
	}

	return summary, nil
}

func (r *OrderRepository) VerifyPickup(_ context.Context, vendorUserID string, orderID string, pickupCode string) (*model.VerifyPickupResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	orders := r.vendorOrders[vendorUserID]
	var foundIndex = -1
	for i, o := range orders {
		if o.ID == orderID || strings.EqualFold(o.ID, orderID) {
			foundIndex = i
			break
		}
	}
	if foundIndex == -1 {
		return nil, exception.ErrOrderNotFound
	}

	order := &orders[foundIndex]
	dbStatus := effectiveDBStatus(order)
	if dbStatus == enum.OrderStatusCompleted {
		return nil, exception.ErrOrderAlreadyDelivered
	}
	if !enum.CanVendorTransition(dbStatus, enum.OrderStatusCompleted) {
		return nil, exception.ErrOrderCannotBeDelivered
	}
	if pickupCode != "" && order.PickupCode != "" && pickupCode != order.PickupCode {
		return nil, exception.ErrInvalidPickupCode
	}

	now := time.Now().UTC()
	order.DBStatus = enum.OrderStatusCompleted
	order.Status = enum.VendorOrderStatusDelivered
	order.DeliveredAt = &now

	return &model.VerifyPickupResult{
		Success:     true,
		OrderID:     order.ID,
		Status:      enum.VendorOrderStatusDelivered,
		DeliveredAt: now,
		Message:     "تحویل بسته با موفقیت ثبت شد",
	}, nil
}

func (r *OrderRepository) UpdateVendorOrderStatus(_ context.Context, vendorUserID string, orderID string, targetStatus enum.OrderStatus, _ string) (*model.VendorOrderStatusUpdateResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	orders := r.vendorOrders[vendorUserID]
	foundIndex := -1
	for i, o := range orders {
		if o.ID == orderID || strings.EqualFold(o.ID, orderID) {
			foundIndex = i
			break
		}
	}
	if foundIndex == -1 {
		return nil, exception.ErrOrderNotFound
	}

	order := &orders[foundIndex]
	currentDBStatus := effectiveDBStatus(order)
	if !enum.CanVendorTransition(currentDBStatus, targetStatus) {
		return nil, exception.ErrInvalidOrderTransition
	}

	now := time.Now().UTC()
	order.DBStatus = targetStatus
	order.Status = targetStatus.ToVendorStatus()

	if targetStatus == enum.OrderStatusCancelled && order.Package.ID != "" {
		r.offerStock[order.Package.ID] += order.Quantity
	}

	return &model.VendorOrderStatusUpdateResult{
		Success:   true,
		OrderID:   order.ID,
		Status:    order.Status,
		UpdatedAt: now,
		Message:   "وضعیت سفارش با موفقیت به‌روزرسانی شد",
	}, nil
}

func effectiveDBStatus(o *model.VendorOrder) enum.OrderStatus {
	if o.DBStatus != "" {
		return o.DBStatus
	}
	switch o.Status {
	case enum.VendorOrderStatusReadyForPickup:
		return enum.OrderStatusReady
	case enum.VendorOrderStatusDelivered:
		return enum.OrderStatusCompleted
	case enum.VendorOrderStatusCancelled:
		return enum.OrderStatusCancelled
	default:
		return enum.OrderStatusPaid
	}
}
