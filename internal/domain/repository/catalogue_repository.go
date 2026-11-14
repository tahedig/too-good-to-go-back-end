package repository

import (
	"context"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
)

// CatalogueRepository serves the customer-facing browsing screens. Every read goes through the
// eligible_offers view, so "can a customer see this offer?" is decided in exactly one place.
type CatalogueRepository interface {
	Categories(ctx context.Context) ([]model.CatalogueCategory, error)
	Search(ctx context.Context, p model.CatalogueSearch) ([]model.CatalogueOffer, error)
	Favorites(ctx context.Context, p model.CatalogueFavoritesSearch) ([]model.CatalogueOffer, error)
	Home(ctx context.Context, section model.HomeSection, p model.HomeSearch) ([]model.HomeOffer, error)
	Map(ctx context.Context, p model.CatalogueMapSearch) ([]model.MapOffer, error)
	FilterData(ctx context.Context) (model.CatalogueFilterData, error)

	// AddFavorite is idempotent. found=false means the offer is not visible to customers.
	AddFavorite(ctx context.Context, userID int64, offerPublicID string) (found bool, err error)
	// RemoveFavorite is idempotent and never reveals whether the offer exists.
	RemoveFavorite(ctx context.Context, userID int64, offerPublicID string) error

	UserExists(ctx context.Context, userID int64) (bool, error)
}
