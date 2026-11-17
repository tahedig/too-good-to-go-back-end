package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
)

type fakeCatalogueRepo struct {
	search   []model.CatalogueOffer
	lastSrch model.CatalogueSearch
	lastHome model.HomeSearch
	homeCall int
	addFound bool
}

func (f *fakeCatalogueRepo) Categories(context.Context) ([]model.CatalogueCategory, error) {
	return []model.CatalogueCategory{{ID: 1, Slug: "bakery", NameFA: "نانوایی", Kind: "bakery"}}, nil
}
func (f *fakeCatalogueRepo) Search(_ context.Context, p model.CatalogueSearch) ([]model.CatalogueOffer, error) {
	f.lastSrch = p
	return f.search, nil
}
func (f *fakeCatalogueRepo) Favorites(context.Context, model.CatalogueFavoritesSearch) ([]model.CatalogueOffer, error) {
	return nil, nil
}
func (f *fakeCatalogueRepo) Home(_ context.Context, _ model.HomeSection, p model.HomeSearch) ([]model.HomeOffer, error) {
	f.lastHome = p
	f.homeCall++
	return nil, nil
}
func (f *fakeCatalogueRepo) Map(context.Context, model.CatalogueMapSearch) ([]model.MapOffer, error) {
	return nil, nil
}
func (f *fakeCatalogueRepo) FilterData(context.Context) (model.CatalogueFilterData, error) {
	return model.CatalogueFilterData{PriceMin: 40000, PriceMax: 70000, OfferCount: 22,
		Allergens: []model.CatalogueAllergen{{Code: "gluten", NameFA: "گلوتن"}}}, nil
}
func (f *fakeCatalogueRepo) AddFavorite(context.Context, int64, string) (bool, error) {
	return f.addFound, nil
}
func (f *fakeCatalogueRepo) RemoveFavorite(context.Context, int64, string) error { return nil }
func (f *fakeCatalogueRepo) UserExists(context.Context, int64) (bool, error)     { return true, nil }

func catRows(n int) []model.CatalogueOffer {
	out := make([]model.CatalogueOffer, n)
	for i := range out {
		out[i] = model.CatalogueOffer{ID: int64(i + 1), SortKey: int64(100 + i), PublicID: "p", Title: "t",
			PickupStart: time.Now(), PickupEnd: time.Now().Add(time.Hour)}
	}
	return out
}

func catF64(v float64) *float64 { return &v }
func catI64(v int64) *int64     { return &v }
func catInt(v int) *int         { return &v }

func TestSearchDefaultsAndPaging(t *testing.T) {
	repo := &fakeCatalogueRepo{search: catRows(21)} // repo returns limit+1 => has more
	svc := NewCatalogueService(repo)

	res, err := svc.SearchOffers(context.Background(), dto.CatalogueQuery{Q: "  سنگك  "})
	if err != nil {
		t.Fatal(err)
	}
	if repo.lastSrch.Limit != 21 || repo.lastSrch.Sort != "newest" || repo.lastSrch.Query != "سنگک" {
		t.Fatalf("unexpected repo input: %+v", repo.lastSrch)
	}
	if len(res.Items) != 20 || !res.HasMore || res.NextCursor == nil {
		t.Fatalf("want 20 items + cursor, got %d has_more=%v", len(res.Items), res.HasMore)
	}

	// the cursor is accepted for the same sort and carries the last row's key and id
	if _, err := svc.SearchOffers(context.Background(), dto.CatalogueQuery{Cursor: *res.NextCursor}); err != nil {
		t.Fatalf("cursor rejected: %v", err)
	}
	if c := repo.lastSrch.Cursor; c == nil || c.K1 != 119 || c.ID != 20 {
		t.Fatalf("cursor not forwarded: %+v", repo.lastSrch.Cursor)
	}
	// ...but not when the sort changes
	_, err = svc.SearchOffers(context.Background(), dto.CatalogueQuery{Cursor: *res.NextCursor, Sort: "price_asc"})
	if err == nil || !strings.Contains(err.Error(), "cursor") {
		t.Fatalf("cursor from another sort must be rejected, got %v", err)
	}
}

func TestSearchDefaultsToNearestWithLocation(t *testing.T) {
	repo := &fakeCatalogueRepo{}
	svc := NewCatalogueService(repo)
	if _, err := svc.SearchOffers(context.Background(), dto.CatalogueQuery{Lat: catF64(35.7), Lng: catF64(51.4)}); err != nil {
		t.Fatal(err)
	}
	if repo.lastSrch.Sort != "nearest" {
		t.Fatalf("want nearest, got %s", repo.lastSrch.Sort)
	}
}

func TestSearchValidation(t *testing.T) {
	svc := NewCatalogueService(&fakeCatalogueRepo{})
	_, err := svc.SearchOffers(context.Background(), dto.CatalogueQuery{
		Sort: "nearest", Kind: "pizza", ExcludeAllergens: []string{"lava"}, CategoryIDs: []int{0},
		PriceMin: catI64(500), PriceMax: catI64(100), Limit: catInt(500), RadiusKm: catF64(3),
	})
	if err == nil {
		t.Fatal("expected a validation error")
	}
	for _, want := range []string{"sort", "kind", "exclude_allergens", "category", "price_min", "limit", "radius_km"} {
		if !strings.Contains(err.Error(), want+":") {
			t.Errorf("error does not mention %q: %v", want, err)
		}
	}
}

func TestStoreOffersRejectsBadUUID(t *testing.T) {
	svc := NewCatalogueService(&fakeCatalogueRepo{})
	if _, err := svc.StoreOffers(context.Background(), "abc", dto.CatalogueQuery{}); err == nil {
		t.Fatal("expected error")
	}
	repo := &fakeCatalogueRepo{}
	svc = NewCatalogueService(repo)
	if _, err := svc.StoreOffers(context.Background(), "3F2B8F0E-6C1D-4C53-9A43-0D1D5C7A9E11", dto.CatalogueQuery{}); err != nil {
		t.Fatal(err)
	}
	if repo.lastSrch.StorePublicID != "3f2b8f0e-6c1d-4c53-9a43-0d1d5c7a9e11" {
		t.Fatalf("store id not canonicalised: %q", repo.lastSrch.StorePublicID)
	}
}

func TestHomeDefaults(t *testing.T) {
	repo := &fakeCatalogueRepo{}
	svc := NewCatalogueService(repo)
	// popular-nearby without a location is NOT an error (the SQL returns zero rows)
	if _, err := svc.HomePopularNearby(context.Background(), dto.HomeQuery{}); err != nil {
		t.Fatal(err)
	}
	if repo.lastHome.Limit != 11 || repo.lastHome.RadiusKm != 8 {
		t.Fatalf("want limit 10+1 and radius 8, got %+v", repo.lastHome)
	}
	if _, err := svc.HomeToday(context.Background(), dto.HomeQuery{Limit: catInt(31)}); err == nil {
		t.Fatal("limit above 30 must be rejected")
	}
	if _, err := svc.HomeLastChance(context.Background(), dto.HomeQuery{Cursor: "garbage"}); err == nil {
		t.Fatal("garbage cursor must be rejected")
	}
}

func TestMapRequiresLocation(t *testing.T) {
	svc := NewCatalogueService(&fakeCatalogueRepo{})
	if _, err := svc.MapOffers(context.Background(), dto.MapQuery{}); err == nil {
		t.Fatal("expected error without lat/lng")
	}
	if _, err := svc.MapOffers(context.Background(), dto.MapQuery{Lat: catF64(95), Lng: catF64(51)}); err == nil {
		t.Fatal("expected error for latitude out of range")
	}
	if _, err := svc.MapOffers(context.Background(), dto.MapQuery{Lat: catF64(35.7), Lng: catF64(51.4)}); err != nil {
		t.Fatal(err)
	}
}

func TestFavorites(t *testing.T) {
	repo := &fakeCatalogueRepo{addFound: false}
	svc := NewCatalogueService(repo)
	if _, err := svc.AddFavorite(context.Background(), 2, "nope"); err == nil {
		t.Fatal("bad uuid must be rejected")
	}
	if _, err := svc.AddFavorite(context.Background(), 2, "3f2b8f0e-6c1d-4c53-9a43-0d1d5c7a9e11"); err == nil {
		t.Fatal("unknown offer must be a 404")
	}
	repo.addFound = true
	res, err := svc.AddFavorite(context.Background(), 2, "3f2b8f0e-6c1d-4c53-9a43-0d1d5c7a9e11")
	if err != nil || !res.IsFavorite {
		t.Fatalf("add favorite failed: %v %+v", err, res)
	}
	if err := svc.RemoveFavorite(context.Background(), 2, "3f2b8f0e-6c1d-4c53-9a43-0d1d5c7a9e11"); err != nil {
		t.Fatal(err)
	}
}

func TestPickupWindowText(t *testing.T) {
	now := time.Date(2026, 11, 13, 10, 0, 0, 0, tehranLoc)
	start := time.Date(2026, 11, 13, 18, 0, 0, 0, tehranLoc)
	end := time.Date(2026, 11, 13, 19, 0, 0, 0, tehranLoc)
	if got := pickupWindowText(start, end, now); got != "امروز، ۱۸:۰۰-۱۹:۰۰" {
		t.Fatalf("got %q", got)
	}
	if got := pickupWindowText(start.AddDate(0, 0, 1), end.AddDate(0, 0, 1), now); got != "فردا، ۱۸:۰۰-۱۹:۰۰" {
		t.Fatalf("got %q", got)
	}
}
