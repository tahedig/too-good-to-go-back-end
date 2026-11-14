package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
)

// Smoke test: runs every generated statement with realistic arguments against a real database, so
// parameter typing problems (the one thing the psql-based SQL tests cannot see) show up in CI/dev.
//
//	TEST_DATABASE_URL=postgres://tgtg:tgtg@localhost:5432/tgtg_dev?sslmode=disable go test ./internal/infrastructure/repository/postgres/
//
// Skipped when TEST_DATABASE_URL is not set. Needs migrations 000001-000009 (data is optional).
func TestCatalogueRepositorySmoke(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	repo := NewCatalogueRepository(pool)

	lat, lng, radius := 35.7219, 51.4090, 8.0
	user := int64(2)
	pmin, pmax := int64(0), int64(1_000_000)
	from, to := time.Now().Add(-time.Hour), time.Now().Add(48*time.Hour)
	const someUUID = "3f2b8f0e-6c1d-4c53-9a43-0d1d5c7a9e11"

	t.Run("categories+filters", func(t *testing.T) {
		if _, err := repo.Categories(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.FilterData(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("search: bare", func(t *testing.T) {
		if _, err := repo.Search(ctx, model.CatalogueSearch{Sort: "newest", Limit: 11}); err != nil {
			t.Fatal(err)
		}
	})

	for _, sort := range []string{"nearest", "price_asc", "price_desc", "newest"} {
		sort := sort
		t.Run("search: everything, sort="+sort, func(t *testing.T) {
			first, err := repo.Search(ctx, model.CatalogueSearch{
				Query: `100%_off\ نان`, CategoryIDs: []int16{1, 7}, Kind: "bakery",
				PriceMin: &pmin, PriceMax: &pmax, PickupFrom: &from, PickupTo: &to,
				ExcludeAllergens: []string{"gluten", "dairy"}, StorePublicID: someUUID,
				Lat: &lat, Lng: &lng, RadiusKm: &radius, Sort: sort, UserID: &user, Limit: 11,
			})
			if err != nil {
				t.Fatal(err)
			}
			_ = first
			// with a cursor
			if _, err := repo.Search(ctx, model.CatalogueSearch{
				Lat: &lat, Lng: &lng, Sort: sort, Cursor: &model.CatalogueCursor{K1: 0, ID: 0}, Limit: 11,
			}); err != nil {
				t.Fatal(err)
			}
		})
	}

	for _, sec := range []model.HomeSection{model.HomeSectionToday, model.HomeSectionPopularNearby, model.HomeSectionLastChance} {
		sec := sec
		t.Run("home: "+string(sec), func(t *testing.T) {
			for _, p := range []model.HomeSearch{
				{Lat: &lat, Lng: &lng, RadiusKm: radius, UserID: &user, Limit: 11},
				{RadiusKm: radius, Limit: 11}, // no location, anonymous
				{Lat: &lat, Lng: &lng, RadiusKm: radius, Cursor: &model.CatalogueCursor{K1: -1, K2: 0, ID: 0}, Limit: 11},
			} {
				if _, err := repo.Home(ctx, sec, p); err != nil {
					t.Fatal(err)
				}
			}
		})
	}

	t.Run("map", func(t *testing.T) {
		if _, err := repo.Map(ctx, model.CatalogueMapSearch{Lat: lat, Lng: lng, RadiusKm: 5, Limit: 100}); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.Map(ctx, model.CatalogueMapSearch{
			Lat: lat, Lng: lng, RadiusKm: 5, CategoryIDs: []int16{1}, Kind: "bakery", UserID: &user, Limit: 10,
		}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("favorites", func(t *testing.T) {
		if _, err := repo.Favorites(ctx, model.CatalogueFavoritesSearch{UserID: user, Limit: 11}); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.Favorites(ctx, model.CatalogueFavoritesSearch{
			UserID: user, Lat: &lat, Lng: &lng, Cursor: &model.CatalogueCursor{K1: 0, ID: 0}, Limit: 11,
		}); err != nil {
			t.Fatal(err)
		}
		found, err := repo.AddFavorite(ctx, user, someUUID) // unknown offer: must be (false, nil)
		if err != nil || found {
			t.Fatalf("unknown offer: found=%v err=%v", found, err)
		}
		if err := repo.RemoveFavorite(ctx, user, someUUID); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("user exists", func(t *testing.T) {
		if ok, err := repo.UserExists(ctx, 987654321); err != nil || ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
	})
}
