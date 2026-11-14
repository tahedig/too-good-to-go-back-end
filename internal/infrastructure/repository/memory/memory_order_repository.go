package memory

import (
	"context"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

// OrderRepository keeps orders in memory. Nothing creates orders yet, so in
// development it starts empty; tests fill it with Add.
type OrderRepository struct {
	mu     sync.Mutex
	byUser map[string][]model.Order
}

func NewOrderRepository() *OrderRepository {
	return &OrderRepository{byUser: make(map[string][]model.Order)}
}

var _ repository.OrderRepository = (*OrderRepository)(nil)

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
