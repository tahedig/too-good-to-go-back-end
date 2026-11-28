package repository

import (
	"context"
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
)

// AdminDashboardQuery configures the admin dashboard summary query.
type AdminDashboardQuery struct {
	// ActivityLimit is the maximum number of recent-activity items to return.
	ActivityLimit int
	// ActivityTypes restricts the recent-activity feed to these types; nil or
	// empty includes all activity types.
	ActivityTypes []model.AdminActivityType
	// Now is the reference timestamp used for active offer eligibility.
	Now time.Time
}

// AdminDashboardRepository aggregates platform totals and recent activity for
// the admin dashboard.
type AdminDashboardRepository interface {
	GetDashboard(ctx context.Context, q AdminDashboardQuery) (*model.AdminDashboard, error)
}
