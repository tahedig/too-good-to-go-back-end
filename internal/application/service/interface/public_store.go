package serviceinterface

import (
	"context"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
)

type PublicStoreService interface {
	GetPublicStore(ctx context.Context, publicID string) (*dto.PublicStoreResponse, error)
}
