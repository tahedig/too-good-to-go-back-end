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

type AdminStoreListInput struct {
	Search         string
	Status         string
	CategoryID     *int
	City           string
	AcceptsOrders  *bool
	IncludeDeleted *bool
	Sort           string
	Limit          *int
	Cursor         string
}

type AdminStoreListPage struct {
	Items      []*model.Store
	Total      int
	HasMore    bool
	NextCursor *string
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

type AdminStoreManagementService interface {
	ListStores(ctx context.Context, in AdminStoreListInput) (*AdminStoreListPage, error)
	GetManagedStore(ctx context.Context, storeID string) (*model.Store, error)
	UpdateManagedStore(ctx context.Context, storeID string, updated *model.AdminStoreUpdate) (*model.Store, error)
}
