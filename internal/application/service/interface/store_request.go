package serviceinterface

import (
	"context"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
)

type StoreRequestService interface {
	ListStoreRequests(ctx context.Context, userID int64) ([]*model.StoreRequest, error)
	GetStoreRequest(ctx context.Context, userID int64, publicID string) (*model.StoreRequest, error)
	CreateStoreRequest(ctx context.Context, userID int64, req *model.StoreRequest) (*model.StoreRequest, error)
}
