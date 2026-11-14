package memory

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

// VendorApprovalRepository keeps stores in memory, with the same rules as
// PostgreSQL: conditional transitions and a decision history. Nothing
// creates stores yet, so in development it starts empty; tests use Add.
type VendorApprovalRepository struct {
	mu      sync.Mutex
	stores  map[string]*model.VendorStore
	history []model.StoreTransition
	now     func() time.Time
}

func NewVendorApprovalRepository() *VendorApprovalRepository {
	return &VendorApprovalRepository{stores: make(map[string]*model.VendorStore), now: time.Now}
}

var _ repository.VendorApprovalRepository = (*VendorApprovalRepository)(nil)

// Add stores a store as it would exist in the database.
func (r *VendorApprovalRepository) Add(s model.VendorStore) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := s
	r.stores[s.ID] = &cp
}

// History returns every recorded decision, oldest first.
func (r *VendorApprovalRepository) History() []model.StoreTransition {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]model.StoreTransition(nil), r.history...)
}

func (r *VendorApprovalRepository) List(_ context.Context, q repository.VendorStoreQuery) (*repository.VendorStorePage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	page := &repository.VendorStorePage{Items: []model.VendorStore{}, Counts: map[enum.StoreStatus]int{}}
	for _, s := range enum.StoreStatuses {
		page.Counts[s] = 0
	}
	var matched []model.VendorStore
	search := strings.ToLower(q.Search)
	for _, s := range r.stores {
		page.Counts[s.Status]++
		if len(q.Statuses) > 0 && !containsStatus(q.Statuses, s.Status) {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(strings.Join(
			[]string{s.StoreName, s.OwnerName, s.Category, s.Address}, "\n")), search) {
			continue
		}
		matched = append(matched, *s)
	}
	sort.Slice(matched, func(i, j int) bool {
		return newerThan(matched[i].SubmittedAt, matched[i].ID, matched[j].SubmittedAt, matched[j].ID)
	})
	page.Total = len(matched)
	if q.Offset < len(matched) {
		end := min(q.Offset+q.Limit, len(matched))
		page.Items = matched[q.Offset:end]
	}
	return page, nil
}

func (r *VendorApprovalRepository) Get(_ context.Context, storeID string) (*model.VendorStore, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.stores[storeID]
	if !ok {
		return nil, repository.ErrStoreNotFound
	}
	cp := *s
	return &cp, nil
}

func (r *VendorApprovalRepository) Transition(_ context.Context, t model.StoreTransition) (*model.VendorStore, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.stores[t.StoreID]
	if !ok {
		return nil, repository.ErrStoreNotFound
	}
	if s.Status != t.From {
		return nil, repository.ErrStoreStatusChanged
	}
	now := r.now().UTC()
	s.Status = t.To
	s.StatusUpdatedAt = &now
	s.Decision = nil
	if t.ReasonCode != "" {
		s.Decision = &model.StoreDecision{
			Action: t.Action, ReasonCode: t.ReasonCode, Message: t.Message, DecidedAt: now, DecidedBy: t.AdminUserID,
		}
	}
	r.history = append(r.history, t)
	cp := *s
	return &cp, nil
}

func containsStatus(list []enum.StoreStatus, s enum.StoreStatus) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
