package serviceinterface

import (
	"context"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
)

// AdminDashboardInput holds raw query parameters for GET /admin/dashboard;
// the service validates them before querying the repository.
type AdminDashboardInput struct {
	ActivityLimit string // optional 1-50 (default 10)
	ActivityType  string // optional comma-separated: store_application, store_decision, reservation
}

// AdminDashboardService builds the admin-only platform summary and activity feed.
type AdminDashboardService interface {
	GetDashboard(ctx context.Context, in AdminDashboardInput) (*model.AdminDashboard, error)
}
