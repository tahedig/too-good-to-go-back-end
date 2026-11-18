package repository

import (
	"context"
	"errors"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
)

var ErrAdminStoreNotFound = errors.New("admin store not found")

// AdminStoreRepository provides admin-level read and status operations for stores.
type AdminStoreRepository interface {
	GetStore(ctx context.Context, storeID string) (*model.AdminStoreDetail, error)
	UpdateStoreStatus(ctx context.Context, storeID string, isActive bool) (*model.AdminStoreDetail, error)
}
