package serviceinterface

import (
	"context"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
)

type AdminStoreService interface {
	GetStore(ctx context.Context, storeID string) (*model.AdminStoreDetail, error)
	UpdateStoreStatus(ctx context.Context, storeID string, isActive *bool) (*model.AdminStoreDetail, error)
}
