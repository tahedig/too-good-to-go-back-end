package repository

import (
	"context"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
)

// OfferRepository defines persistence operations for food offers and catalog queries.
type OfferRepository interface {
	// ListAvailableByStoreID returns all active, in-stock offers for a store.
	ListAvailableByStoreID(ctx context.Context, storeID int64) ([]*model.Offer, error)
}
