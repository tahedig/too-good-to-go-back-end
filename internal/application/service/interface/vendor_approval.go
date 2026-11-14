package serviceinterface

import (
	"context"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
)

// VendorListInput is the raw query of the admin vendor list; the service
// validates it.
type VendorListInput struct {
	Status   string // comma-separated store statuses; empty = all
	Search   string
	Page     string // 1-based; empty = 1
	PageSize string // empty = default
}

// VendorDecisionInput is the body of reject and suspend.
type VendorDecisionInput struct {
	ReasonCode string
	Message    string
}

type VendorStoreList struct {
	Items    []model.VendorStore
	Total    int
	Page     int
	PageSize int
	Counts   map[enum.StoreStatus]int
}

// VendorApprovalService lets admins review stores. Callers must have
// checked that the caller is an admin.
type VendorApprovalService interface {
	ListVendors(ctx context.Context, in VendorListInput) (*VendorStoreList, error)
	GetVendor(ctx context.Context, storeID string) (*model.VendorStore, error)
	// Decide applies an admin decision. adminID is the admin's public ID
	// from the token, never from the request body.
	Decide(ctx context.Context, adminID, storeID string, action enum.StoreAction, in VendorDecisionInput) (*model.VendorStore, error)
}
