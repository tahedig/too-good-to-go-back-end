package repository

import (
	"context"
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
)

type AdminStoreFilter struct {
	Search         string
	Status         string
	CategoryID     *int16
	City           string
	AcceptsOrders  *bool
	IncludeDeleted bool
	Sort           string
	Cursor         *AdminStoreCursor
	Limit          int
}

type AdminStoreCursor struct {
	CreatedAt time.Time
	ID        int64
}

type AdminStorePage struct {
	Items []*model.Store
	Total int
}

// AdminStoreManagementRepository provides unscoped store reads for admins.
// Update persists only profile fields; identity and approval columns are never written.
type AdminStoreManagementRepository interface {
	ListAdmin(ctx context.Context, filter AdminStoreFilter) (*AdminStorePage, error)
	GetByPublicID(ctx context.Context, publicID string) (*model.Store, error)
	CategoryIsActive(ctx context.Context, categoryID int16) (bool, error)
	Update(ctx context.Context, store *model.Store) error
}
