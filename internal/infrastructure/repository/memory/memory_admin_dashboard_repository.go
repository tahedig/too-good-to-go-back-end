package memory

import (
	"context"
	"slices"
	"sort"
	"sync"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

// AdminDashboardRepository holds admin dashboard fixtures in memory for
// development without PostgreSQL and for HTTP/service tests.
type AdminDashboardRepository struct {
	mu           sync.Mutex
	totals       model.AdminDashboardTotals
	applications model.AdminApplicationBreakdown
	reservations model.AdminReservationBreakdown
	activity     []model.AdminActivityItem
}

func NewAdminDashboardRepository() *AdminDashboardRepository {
	return &AdminDashboardRepository{}
}

var _ repository.AdminDashboardRepository = (*AdminDashboardRepository)(nil)

func (r *AdminDashboardRepository) SetSnapshot(
	totals model.AdminDashboardTotals,
	applications model.AdminApplicationBreakdown,
	reservations model.AdminReservationBreakdown,
) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.totals = totals
	r.applications = applications
	r.reservations = reservations
}

func (r *AdminDashboardRepository) AddActivity(item model.AdminActivityItem) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item.OccurredAt = item.OccurredAt.UTC()
	r.activity = append(r.activity, item)
}

func (r *AdminDashboardRepository) GetDashboard(_ context.Context, q repository.AdminDashboardQuery) (*model.AdminDashboard, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	filtered := make([]model.AdminActivityItem, 0, len(r.activity))
	for _, item := range r.activity {
		if len(q.ActivityTypes) > 0 && !slices.Contains(q.ActivityTypes, item.Type) {
			continue
		}
		cp := item
		if item.Amount != nil {
			v := *item.Amount
			cp.Amount = &v
		}
		filtered = append(filtered, cp)
	}

	sort.Slice(filtered, func(i, j int) bool {
		return newerThan(filtered[i].OccurredAt, filtered[i].EntityID, filtered[j].OccurredAt, filtered[j].EntityID)
	})

	limit := q.ActivityLimit
	if limit <= 0 || limit > len(filtered) {
		limit = len(filtered)
	}

	return &model.AdminDashboard{
		Totals:         r.totals,
		Applications:   r.applications,
		Reservations:   r.reservations,
		RecentActivity: filtered[:limit],
	}, nil
}
