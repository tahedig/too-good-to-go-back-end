package serviceinterface

import (
	"context"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
)

type AdminStoreService interface {
	GetStore(ctx context.Context, storeID string) (*model.AdminStoreDetail, error)
	UpdateStoreStatus(ctx context.Context, storeID string, isActive *bool) (*model.AdminStoreDetail, error)
	GetStoreSubscription(ctx context.Context, storeID string) (*model.AdminStoreSubscription, error)
	ListStoreOffers(ctx context.Context, storeID string) ([]model.AdminOffer, error)
	GetOffer(ctx context.Context, offerID string) (*model.AdminOffer, error)
	UpdateOfferStatus(ctx context.Context, offerID string, isActive *bool) (*model.AdminOffer, error)
}
