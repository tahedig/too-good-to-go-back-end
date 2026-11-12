package repository

import (
	"context"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
)

// StoreRequestRepository defines persistence operations for vendor store creation requests.
type StoreRequestRepository interface {
	// ListByUser returns all store requests submitted by the vendor.
	ListByUser(ctx context.Context, userID int64) ([]*model.StoreRequest, error)

	// GetByPublicIDAndUser retrieves a single store request if owned by the vendor.
	GetByPublicIDAndUser(ctx context.Context, publicID string, userID int64) (*model.StoreRequest, error)

	// Create persists a new store request.
	Create(ctx context.Context, req *model.StoreRequest) error
}
