package memory

import (
	"context"
	"sync"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

// AdminStoreRepository keeps admin store details in memory for development and tests.
type AdminStoreRepository struct {
	mu     sync.RWMutex
	stores map[string]*model.AdminStoreDetail
}

func NewAdminStoreRepository() *AdminStoreRepository {
	return &AdminStoreRepository{
		stores: make(map[string]*model.AdminStoreDetail),
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

func cloneAdminStore(s model.AdminStoreDetail) model.AdminStoreDetail {
	cp := s
	if s.WorkingHours != nil {
		cp.WorkingHours = append([]model.AdminStoreWorkingHour(nil), s.WorkingHours...)
	}
	return cp
}
