package serviceinterface

import (
	"context"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
)

// VendorDashboardInput holds raw query parameters for GET /vendor/dashboard;
// the service validates them before querying the repository.
type VendorDashboardInput struct {
	StoreID     string // optional UUID of an owned store
	RecentLimit string // optional 1-50 (default 10)
}

// VendorDashboardService builds the authenticated vendor's dashboard summary.
type VendorDashboardService interface {
	GetDashboard(ctx context.Context, vendorUserID string, in VendorDashboardInput) (*model.VendorDashboard, error)
}
