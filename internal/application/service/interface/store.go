package serviceinterface

import (
	"context"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
)

type StoreService interface {
	ListStores(ctx context.Context, userID int64) ([]*model.Store, error)
	GetStore(ctx context.Context, userID int64, publicID string) (*model.Store, error)
	UpdateStore(ctx context.Context, userID int64, publicID string, updated *model.Store) (*model.Store, error)
}
