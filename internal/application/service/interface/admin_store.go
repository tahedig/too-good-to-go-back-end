package serviceinterface

import (
	"context"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

type AdminStoreOrderListInput struct {
	Status   string
	Page     string
	PageSize string
}

type AdminStoreService interface {
	GetStore(ctx context.Context, storeID string) (*model.AdminStoreDetail, error)
	UpdateStoreStatus(ctx context.Context, storeID string, isActive *bool) (*model.AdminStoreDetail, error)
	GetStoreSubscription(ctx context.Context, storeID string) (*model.AdminStoreSubscription, error)
	ListStoreOffers(ctx context.Context, storeID string) ([]model.AdminOffer, error)
	GetOffer(ctx context.Context, offerID string) (*model.AdminOffer, error)
	UpdateOfferStatus(ctx context.Context, offerID string, isActive *bool) (*model.AdminOffer, error)
	ListStoreOrders(ctx context.Context, storeID string, in AdminStoreOrderListInput) (*repository.AdminStoreOrderPage, error)
}
