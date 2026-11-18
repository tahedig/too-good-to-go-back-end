package memory

import (
	"context"
	"slices"
	"sort"
	"sync"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

// AdminStoreRepository keeps admin store details in memory for development and tests.
type AdminStoreRepository struct {
	mu            sync.RWMutex
	stores        map[string]*model.AdminStoreDetail
	subscriptions map[string]*model.AdminStoreSubscription
	offers        map[string]*model.AdminOffer
	orders        map[string][]model.AdminStoreOrder
}

func NewAdminStoreRepository() *AdminStoreRepository {
	return &AdminStoreRepository{
		stores:        make(map[string]*model.AdminStoreDetail),
		subscriptions: make(map[string]*model.AdminStoreSubscription),
		offers:        make(map[string]*model.AdminOffer),
		orders:        make(map[string][]model.AdminStoreOrder),
	}
}

var _ repository.AdminStoreRepository = (*AdminStoreRepository)(nil)

// AddStore seeds a store into the in-memory repository for tests or local dev.
func (r *AdminStoreRepository) AddStore(s model.AdminStoreDetail) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := cloneAdminStore(s)
	r.stores[s.ID] = &cp
}

func (r *AdminStoreRepository) GetStore(_ context.Context, storeID string) (*model.AdminStoreDetail, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.stores[storeID]
	if !ok {
		return nil, repository.ErrAdminStoreNotFound
	}
	cp := cloneAdminStore(*s)
	return &cp, nil
}

func (r *AdminStoreRepository) UpdateStoreStatus(_ context.Context, storeID string, isActive bool) (*model.AdminStoreDetail, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.stores[storeID]
	if !ok {
		return nil, repository.ErrAdminStoreNotFound
	}
	s.IsActive = isActive
	cp := cloneAdminStore(*s)
	return &cp, nil
}

func (r *AdminStoreRepository) AddSubscription(sub model.AdminStoreSubscription) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := sub
	r.subscriptions[sub.StoreID] = &cp
}

func (r *AdminStoreRepository) GetStoreSubscription(_ context.Context, storeID string) (*model.AdminStoreSubscription, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.stores[storeID]; !ok {
		return nil, repository.ErrAdminStoreNotFound
	}
	sub, ok := r.subscriptions[storeID]
	if !ok {
		return nil, repository.ErrAdminSubscriptionNotFound
	}
	cp := *sub
	return &cp, nil
}

func (r *AdminStoreRepository) AddOffer(offer model.AdminOffer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := offer
	r.offers[offer.ID] = &cp
}

func (r *AdminStoreRepository) ListStoreOffers(_ context.Context, storeID string) ([]model.AdminOffer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.stores[storeID]; !ok {
		return nil, repository.ErrAdminStoreNotFound
	}
	res := make([]model.AdminOffer, 0)
	for _, o := range r.offers {
		if o.StoreID == storeID {
			res = append(res, *o)
		}
	}
	return res, nil
}

func (r *AdminStoreRepository) GetOffer(_ context.Context, offerID string) (*model.AdminOffer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	o, ok := r.offers[offerID]
	if !ok {
		return nil, repository.ErrAdminOfferNotFound
	}
	cp := *o
	return &cp, nil
}

func (r *AdminStoreRepository) UpdateOfferStatus(_ context.Context, offerID string, isActive bool) (*model.AdminOffer, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	o, ok := r.offers[offerID]
	if !ok {
		return nil, repository.ErrAdminOfferNotFound
	}
	o.IsActive = isActive
	if isActive {
		o.Status = "active"
	} else {
		o.Status = "cancelled"
	}
	cp := *o
	return &cp, nil
}

func (r *AdminStoreRepository) AddOrder(order model.AdminStoreOrder) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.orders[order.StoreID] = append(r.orders[order.StoreID], order)
}

func (r *AdminStoreRepository) ListStoreOrders(_ context.Context, storeID string, q repository.AdminStoreOrderQuery) (*repository.AdminStoreOrderPage, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.stores[storeID]; !ok {
		return nil, repository.ErrAdminStoreNotFound
	}
	filtered := make([]model.AdminStoreOrder, 0)
	for _, o := range r.orders[storeID] {
		if len(q.Statuses) > 0 && !slices.Contains(q.Statuses, o.Status) {
			continue
		}
		filtered = append(filtered, o)
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	total := len(filtered)
	start := (q.Page - 1) * q.PageSize
	if start > total {
		start = total
	}
	end := start + q.PageSize
	if end > total {
		end = total
	}
	return &repository.AdminStoreOrderPage{
		Items:    filtered[start:end],
		Total:    total,
		Page:     q.Page,
		PageSize: q.PageSize,
	}, nil
}

func cloneAdminStore(s model.AdminStoreDetail) model.AdminStoreDetail {
	cp := s
	if s.WorkingHours != nil {
		cp.WorkingHours = append([]model.AdminStoreWorkingHour(nil), s.WorkingHours...)
	}
	return cp
}
