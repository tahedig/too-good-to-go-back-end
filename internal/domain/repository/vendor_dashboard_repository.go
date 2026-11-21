package repository

import (
	"context"
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
)

// VendorDashboardQuery specifies the authenticated vendor and optional store
// filter for loading the vendor dashboard summary.
type VendorDashboardQuery struct {
	// VendorUserID is the authenticated vendor's public ID (UUID).
	VendorUserID string
	// StoreID optionally narrows results to a single owned store public ID.
	StoreID string
	// RecentLimit is the maximum number of recent reservations to return.
	RecentLimit int
	// Now is the reference timestamp used for "today" (Asia/Tehran), active
	// offer eligibility, and plan period calculations.
	Now time.Time
}

// VendorDashboardRepository aggregates vendor dashboard metrics and recent
// reservations scoped strictly to stores owned by the authenticated vendor.
type VendorDashboardRepository interface {
	// GetDashboard returns the dashboard summary for q.VendorUserID. When
	// q.StoreID is non-empty and does not exist or is not owned by
	// q.VendorUserID, it returns ErrStoreNotFound.
	GetDashboard(ctx context.Context, q VendorDashboardQuery) (*model.VendorDashboard, error)
}
