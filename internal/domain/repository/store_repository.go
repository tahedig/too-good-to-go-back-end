package repository

import (
	"context"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
)

// StoreRepository defines persistence operations for stores owned by a user.
type StoreRepository interface {
	// ListByUser returns all stores belonging to the user's vendor account(s).
	ListByUser(ctx context.Context, userID int64) ([]*model.Store, error)

	// GetByPublicIDAndUser retrieves a store only if it belongs to the user.
	GetByPublicIDAndUser(ctx context.Context, publicID string, userID int64) (*model.Store, error)

	// Update persists changes to an existing store.
	Update(ctx context.Context, store *model.Store) error
}
