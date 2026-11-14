package serviceinterface

import (
	"context"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
)

// CatalogueService powers the home sections, the offer catalogue, the map and favorite offers.
type CatalogueService interface {
	HomeCategories(ctx context.Context) (*dto.HomeCategoriesResponse, error)
	HomeToday(ctx context.Context, q dto.HomeQuery) (*dto.CatalogueOfferList, error)
	HomePopularNearby(ctx context.Context, q dto.HomeQuery) (*dto.CatalogueOfferList, error)
	HomeLastChance(ctx context.Context, q dto.HomeQuery) (*dto.CatalogueOfferList, error)

	SearchOffers(ctx context.Context, q dto.CatalogueQuery) (*dto.CatalogueOfferList, error)
	StoreOffers(ctx context.Context, storePublicID string, q dto.CatalogueQuery) (*dto.CatalogueOfferList, error)
	Filters(ctx context.Context) (*dto.CatalogueFiltersResponse, error)
	MapOffers(ctx context.Context, q dto.MapQuery) (*dto.CatalogueMapResponse, error)

	ListFavorites(ctx context.Context, userID int64, q dto.FavoritesQuery) (*dto.CatalogueOfferList, error)
	AddFavorite(ctx context.Context, userID int64, offerPublicID string) (*dto.FavoriteOfferResponse, error)
	RemoveFavorite(ctx context.Context, userID int64, offerPublicID string) error

	// UserExists is used by the development auth middleware.
	UserExists(ctx context.Context, userID int64) (bool, error)
}
