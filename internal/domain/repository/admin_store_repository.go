package repository

import (
	"context"
	"errors"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
)

var (
	ErrAdminStoreNotFound        = errors.New("admin store not found")
	ErrAdminSubscriptionNotFound = errors.New("admin subscription not found")
	ErrAdminOfferNotFound        = errors.New("admin offer not found")
)

// AdminStoreRepository provides admin-level read and status operations for stores.
type AdminStoreRepository interface {
	GetStore(ctx context.Context, storeID string) (*model.AdminStoreDetail, error)
	UpdateStoreStatus(ctx context.Context, storeID string, isActive bool) (*model.AdminStoreDetail, error)
	GetStoreSubscription(ctx context.Context, storeID string) (*model.AdminStoreSubscription, error)
	ListStoreOffers(ctx context.Context, storeID string) ([]model.AdminOffer, error)
	GetOffer(ctx context.Context, offerID string) (*model.AdminOffer, error)
	UpdateOfferStatus(ctx context.Context, offerID string, isActive bool) (*model.AdminOffer, error)
}
