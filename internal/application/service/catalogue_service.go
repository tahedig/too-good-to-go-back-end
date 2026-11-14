package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

const (
	catDefaultLimit = 20
	catMaxLimit     = 50

	homeDefaultLimit = 10
	homeMaxLimit     = 30

	mapDefaultLimit = 100
	mapMaxLimit     = 300

	catMaxRadiusKm      = 100.0
	homeDefaultRadiusKm = 8.0 // same radius the SQL tests use for "popular nearby"
	mapDefaultRadiusKm  = 5.0
	catMaxQueryLen      = 100
	catMaxListItems     = 20
	catMaxCategoryID    = 32767
	catCursorMaxEncoded = 200
	catSortNearest      = "nearest"
	catSortPriceAsc     = "price_asc"
	catSortPriceDesc    = "price_desc"
	catSortNewest       = "newest"
	catCursorScopeFavs  = "favorites"
	catKindBakery       = "bakery"
	catKindReadyMeal    = "ready_meal"
	catKindGrocery      = "grocery"
)

var catAllergenCodes = []string{"gluten", "dairy", "egg", "peanut", "nuts", "soy", "fish", "shellfish", "sesame"}

type catalogueService struct {
	repo repository.CatalogueRepository
	now  func() time.Time
}

// NewCatalogueService creates a CatalogueService.
func NewCatalogueService(repo repository.CatalogueRepository) serviceinterface.CatalogueService {
	return &catalogueService{repo: repo, now: time.Now}
}

// ---------------------------------------------------------------- home

func (s *catalogueService) HomeCategories(ctx context.Context) (*dto.HomeCategoriesResponse, error) {
	rows, err := s.repo.Categories(ctx)
	if err != nil {
		return nil, exception.NewInternalServer(err)
	}
	items := make([]dto.HomeCategoryResponse, 0, len(rows))
	for _, c := range rows {
		items = append(items, dto.HomeCategoryResponse{ID: c.ID, Slug: c.Slug, Name: c.NameFA, Kind: c.Kind})
	}
	return &dto.HomeCategoriesResponse{Items: items}, nil
}

func (s *catalogueService) HomeToday(ctx context.Context, q dto.HomeQuery) (*dto.CatalogueOfferList, error) {
	return s.home(ctx, model.HomeSectionToday, q)
}

func (s *catalogueService) HomePopularNearby(ctx context.Context, q dto.HomeQuery) (*dto.CatalogueOfferList, error) {
	return s.home(ctx, model.HomeSectionPopularNearby, q)
}

func (s *catalogueService) HomeLastChance(ctx context.Context, q dto.HomeQuery) (*dto.CatalogueOfferList, error) {
	return s.home(ctx, model.HomeSectionLastChance, q)
}

func (s *catalogueService) home(ctx context.Context, section model.HomeSection, q dto.HomeQuery) (*dto.CatalogueOfferList, error) {
	v := &catValidator{}
	lat, lng := v.latLng(q.Lat, q.Lng)
	radius := v.radius(q.RadiusKm, homeDefaultRadiusKm)
	limit := v.limit(q.Limit, homeDefaultLimit, homeMaxLimit)
	cur := v.cursor(q.Cursor, string(section))
	if err := v.err(); err != nil {
		return nil, err
	}

	rows, err := s.repo.Home(ctx, section, model.HomeSearch{
		Lat: lat, Lng: lng, RadiusKm: radius, Cursor: cur, UserID: q.UserID, Limit: limit + 1,
	})
	if err != nil {
		return nil, exception.NewInternalServer(err)
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	now := s.now()
	items := make([]dto.CatalogueOfferCard, 0, len(rows))
	for _, r := range rows {
		items = append(items, dto.CatalogueOfferCard{
			ID: r.PublicID, Title: r.Title, ImageURL: r.ImageURL,
			Price: r.Price, OriginalPrice: r.OriginalPrice, DiscountPercent: r.DiscountPercent, Currency: r.Currency,
			QuantityAvailable: r.QuantityAvailable, AvailabilityBadge: availabilityBadge(r.QuantityAvailable),
			PickupStart: r.PickupStart, PickupEnd: r.PickupEnd, PickupWindow: pickupWindowText(r.PickupStart, r.PickupEnd, now),
			DistanceM: r.DistanceM, DistanceText: distanceText(r.DistanceM),
			Store:      dto.CatalogueStoreRef{ID: r.StorePublicID, Name: r.StoreName},
			IsFavorite: r.IsFavorite,
		})
	}
	var next *string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		next = encodeCatCursor(string(section), last.K1, last.K2, last.ID)
	}
	return &dto.CatalogueOfferList{Items: items, HasMore: hasMore, NextCursor: next}, nil
}

// ---------------------------------------------------------------- catalogue

func (s *catalogueService) SearchOffers(ctx context.Context, q dto.CatalogueQuery) (*dto.CatalogueOfferList, error) {
	return s.search(ctx, "", q)
}

func (s *catalogueService) StoreOffers(ctx context.Context, storePublicID string, q dto.CatalogueQuery) (*dto.CatalogueOfferList, error) {
	id, err := uuid.Parse(strings.TrimSpace(storePublicID))
	if err != nil {
		return nil, exception.NewBadRequest("invalid store id: must be a UUID", nil)
	}
	return s.search(ctx, id.String(), q)
}

func (s *catalogueService) search(ctx context.Context, storeID string, q dto.CatalogueQuery) (*dto.CatalogueOfferList, error) {
	v := &catValidator{}
	lat, lng := v.latLng(q.Lat, q.Lng)

	text := normalizeCatQuery(q.Q)
	if len([]rune(text)) > catMaxQueryLen {
		v.add("q", fmt.Sprintf("at most %d characters", catMaxQueryLen))
	}
	cats := v.categoryIDs(q.CategoryIDs)
	kind := v.kind(q.Kind)
	allergens := v.allergens(q.ExcludeAllergens)
	if q.PriceMin != nil && *q.PriceMin < 0 {
		v.add("price_min", "must be >= 0")
	}
	if q.PriceMax != nil && *q.PriceMax < 0 {
		v.add("price_max", "must be >= 0")
	}
	if q.PriceMin != nil && q.PriceMax != nil && *q.PriceMin > *q.PriceMax {
		v.add("price_min", "must not exceed price_max")
	}
	if q.PickupFrom != nil && q.PickupTo != nil && !q.PickupFrom.Before(*q.PickupTo) {
		v.add("pickup_from", "must be earlier than pickup_to")
	}
	var radius *float64
	if q.RadiusKm != nil {
		if lat == nil {
			v.add("radius_km", "requires lat and lng")
		}
		r := v.radius(q.RadiusKm, 0)
		radius = &r
	}

	sort := strings.ToLower(strings.TrimSpace(q.Sort))
	switch sort {
	case "":
		sort = catSortNewest
		if lat != nil {
			sort = catSortNearest
		}
	case catSortNearest:
		if lat == nil {
			v.add("sort", "nearest requires lat and lng")
		}
	case catSortPriceAsc, catSortPriceDesc, catSortNewest:
	default:
		v.add("sort", "must be one of nearest, price_asc, price_desc, newest")
	}
	limit := v.limit(q.Limit, catDefaultLimit, catMaxLimit)
	cur := v.cursor(q.Cursor, "catalogue:"+sort)
	if err := v.err(); err != nil {
		return nil, err
	}

	rows, err := s.repo.Search(ctx, model.CatalogueSearch{
		Query: text, CategoryIDs: cats, Kind: kind, PriceMin: q.PriceMin, PriceMax: q.PriceMax,
		PickupFrom: q.PickupFrom, PickupTo: q.PickupTo, ExcludeAllergens: allergens,
		StorePublicID: storeID, Lat: lat, Lng: lng, RadiusKm: radius, Sort: sort,
		Cursor: cur, UserID: q.UserID, Limit: limit + 1,
	})
	if err != nil {
		return nil, exception.NewInternalServer(err)
	}
	return s.catalogueList(rows, limit, "catalogue:"+sort), nil
}

func (s *catalogueService) catalogueList(rows []model.CatalogueOffer, limit int, scope string) *dto.CatalogueOfferList {
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	now := s.now()
	items := make([]dto.CatalogueOfferCard, 0, len(rows))
	for _, r := range rows {
		items = append(items, dto.CatalogueOfferCard{
			ID: r.PublicID, Title: r.Title, ImageURL: r.ImageURL,
			Price: r.Price, OriginalPrice: r.OriginalPrice, DiscountPercent: r.DiscountPercent, Currency: r.Currency,
			QuantityAvailable: r.QuantityAvailable, AvailabilityBadge: availabilityBadge(r.QuantityAvailable),
			PickupStart: r.PickupStart, PickupEnd: r.PickupEnd, PickupWindow: pickupWindowText(r.PickupStart, r.PickupEnd, now),
			CategorySlug: r.CategorySlug, CategoryName: r.CategoryName,
			DistanceM: r.DistanceM, DistanceText: distanceText(r.DistanceM),
			Store:      dto.CatalogueStoreRef{ID: r.StorePublicID, Name: r.StoreName, LogoURL: r.StoreLogoURL},
			IsFavorite: r.IsFavorite,
		})
	}
	var next *string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		next = encodeCatCursor(scope, last.SortKey, 0, last.ID)
	}
	return &dto.CatalogueOfferList{Items: items, HasMore: hasMore, NextCursor: next}
}

func (s *catalogueService) Filters(ctx context.Context) (*dto.CatalogueFiltersResponse, error) {
	cats, err := s.repo.Categories(ctx)
	if err != nil {
		return nil, exception.NewInternalServer(err)
	}
	data, err := s.repo.FilterData(ctx)
	if err != nil {
		return nil, exception.NewInternalServer(err)
	}
	res := &dto.CatalogueFiltersResponse{
		// Labels of the three home tabs: adjust to the final product copy.
		Kinds: []dto.FilterOption{
			{Value: catKindBakery, Label: "نانوایی و شیرینی"},
			{Value: catKindReadyMeal, Label: "غذای آماده"},
			{Value: catKindGrocery, Label: "مواد غذایی"},
		},
		Categories: make([]dto.HomeCategoryResponse, 0, len(cats)),
		Allergens:  make([]dto.FilterOption, 0, len(data.Allergens)),
		Price:      dto.FilterPriceRange{Min: data.PriceMin, Max: data.PriceMax},
		SortOptions: []dto.FilterOption{
			{Value: catSortNearest, Label: "نزدیک‌ترین"},
			{Value: catSortPriceAsc, Label: "ارزان‌ترین"},
			{Value: catSortPriceDesc, Label: "گران‌ترین"},
			{Value: catSortNewest, Label: "جدیدترین"},
		},
		OfferCount: data.OfferCount,
	}
	for _, c := range cats {
		res.Categories = append(res.Categories, dto.HomeCategoryResponse{ID: c.ID, Slug: c.Slug, Name: c.NameFA, Kind: c.Kind})
	}
	for _, a := range data.Allergens {
		res.Allergens = append(res.Allergens, dto.FilterOption{Value: a.Code, Label: a.NameFA})
	}
	return res, nil
}

func (s *catalogueService) MapOffers(ctx context.Context, q dto.MapQuery) (*dto.CatalogueMapResponse, error) {
	v := &catValidator{}
	if q.Lat == nil || q.Lng == nil {
		v.add("lat", "lat and lng are required")
	}
	lat, lng := v.latLng(q.Lat, q.Lng)
	radius := v.radius(q.RadiusKm, mapDefaultRadiusKm)
	cats := v.categoryIDs(q.CategoryIDs)
	kind := v.kind(q.Kind)
	limit := v.limit(q.Limit, mapDefaultLimit, mapMaxLimit)
	if err := v.err(); err != nil {
		return nil, err
	}

	rows, err := s.repo.Map(ctx, model.CatalogueMapSearch{
		Lat: *lat, Lng: *lng, RadiusKm: radius, CategoryIDs: cats, Kind: kind, UserID: q.UserID, Limit: limit,
	})
	if err != nil {
		return nil, exception.NewInternalServer(err)
	}
	now := s.now()
	items := make([]dto.CatalogueMapOffer, 0, len(rows))
	for _, r := range rows {
		items = append(items, dto.CatalogueMapOffer{
			ID: r.PublicID, Title: r.Title, ImageURL: r.ImageURL,
			Price: r.Price, OriginalPrice: r.OriginalPrice, DiscountPercent: r.DiscountPercent, Currency: r.Currency,
			QuantityAvailable: r.QuantityAvailable, PickupWindow: pickupWindowText(r.PickupStart, r.PickupEnd, now),
			CategorySlug: r.CategorySlug, DistanceM: r.DistanceM, DistanceText: distanceText(r.DistanceM),
			Store:      dto.CatalogueStoreRef{ID: r.StorePublicID, Name: r.StoreName},
			Location:   dto.StoreLocationResponse{Latitude: r.StoreLat, Longitude: r.StoreLng},
			IsFavorite: r.IsFavorite,
		})
	}
	return &dto.CatalogueMapResponse{
		Center:   dto.StoreLocationResponse{Latitude: *lat, Longitude: *lng},
		RadiusKm: radius,
		Items:    items,
	}, nil
}

// ---------------------------------------------------------------- favorites

func (s *catalogueService) ListFavorites(ctx context.Context, userID int64, q dto.FavoritesQuery) (*dto.CatalogueOfferList, error) {
	v := &catValidator{}
	lat, lng := v.latLng(q.Lat, q.Lng)
	limit := v.limit(q.Limit, catDefaultLimit, catMaxLimit)
	cur := v.cursor(q.Cursor, catCursorScopeFavs)
	if err := v.err(); err != nil {
		return nil, err
	}
	rows, err := s.repo.Favorites(ctx, model.CatalogueFavoritesSearch{
		UserID: userID, Lat: lat, Lng: lng, Cursor: cur, Limit: limit + 1,
	})
	if err != nil {
		return nil, exception.NewInternalServer(err)
	}
	return s.catalogueList(rows, limit, catCursorScopeFavs), nil
}

func (s *catalogueService) AddFavorite(ctx context.Context, userID int64, offerPublicID string) (*dto.FavoriteOfferResponse, error) {
	id, err := uuid.Parse(strings.TrimSpace(offerPublicID))
	if err != nil {
		return nil, exception.NewBadRequest("invalid offer id: must be a UUID", nil)
	}
	found, err := s.repo.AddFavorite(ctx, userID, id.String())
	if err != nil {
		return nil, exception.NewInternalServer(err)
	}
	if !found {
		return nil, exception.NewNotFound("offer not found")
	}
	return &dto.FavoriteOfferResponse{OfferID: id.String(), IsFavorite: true}, nil
}

func (s *catalogueService) RemoveFavorite(ctx context.Context, userID int64, offerPublicID string) error {
	id, err := uuid.Parse(strings.TrimSpace(offerPublicID))
	if err != nil {
		return exception.NewBadRequest("invalid offer id: must be a UUID", nil)
	}
	if err := s.repo.RemoveFavorite(ctx, userID, id.String()); err != nil {
		return exception.NewInternalServer(err)
	}
	return nil
}

func (s *catalogueService) UserExists(ctx context.Context, userID int64) (bool, error) {
	ok, err := s.repo.UserExists(ctx, userID)
	if err != nil {
		return false, exception.NewInternalServer(err)
	}
	return ok, nil
}

// ---------------------------------------------------------------- validation

type catValidator struct{ msgs []string }

func (v *catValidator) add(field, msg string) { v.msgs = append(v.msgs, field+": "+msg) }

// err returns one 400 whose message lists every invalid parameter, so the client can fix them in one go.
func (v *catValidator) err() error {
	if len(v.msgs) == 0 {
		return nil
	}
	return exception.NewBadRequest("invalid query parameters: "+strings.Join(v.msgs, "; "), nil)
}

func (v *catValidator) latLng(lat, lng *float64) (*float64, *float64) {
	if lat == nil && lng == nil {
		return nil, nil
	}
	ok := true
	if lat == nil || lng == nil {
		v.add("lat", "lat and lng must be sent together")
		return nil, nil
	}
	if *lat < -90 || *lat > 90 {
		v.add("lat", "must be between -90 and 90")
		ok = false
	}
	if *lng < -180 || *lng > 180 {
		v.add("lng", "must be between -180 and 180")
		ok = false
	}
	if !ok {
		return nil, nil
	}
	return lat, lng
}

func (v *catValidator) radius(r *float64, def float64) float64 {
	if r == nil {
		return def
	}
	if *r <= 0 || *r > catMaxRadiusKm || math.IsNaN(*r) {
		v.add("radius_km", fmt.Sprintf("must be > 0 and <= %.0f", catMaxRadiusKm))
		return def
	}
	return *r
}

func (v *catValidator) limit(l *int, def, max int) int {
	if l == nil {
		return def
	}
	if *l < 1 || *l > max {
		v.add("limit", fmt.Sprintf("must be between 1 and %d", max))
		return def
	}
	return *l
}

func (v *catValidator) categoryIDs(in []int) []int16 {
	if len(in) > catMaxListItems {
		v.add("category", fmt.Sprintf("at most %d values", catMaxListItems))
		return nil
	}
	out := make([]int16, 0, len(in))
	for _, id := range in {
		if id < 1 || id > catMaxCategoryID {
			v.add("category", fmt.Sprintf("invalid category id %d", id))
			continue
		}
		out = append(out, int16(id))
	}
	return out
}

func (v *catValidator) kind(k string) string {
	k = strings.ToLower(strings.TrimSpace(k))
	switch k {
	case "", catKindBakery, catKindReadyMeal, catKindGrocery:
		return k
	}
	v.add("kind", "must be one of bakery, ready_meal, grocery")
	return ""
}

func (v *catValidator) allergens(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, raw := range in {
		a := strings.ToLower(strings.TrimSpace(raw))
		if a == "" || seen[a] {
			continue
		}
		valid := false
		for _, code := range catAllergenCodes {
			if code == a {
				valid = true
				break
			}
		}
		if !valid {
			v.add("exclude_allergens", fmt.Sprintf("unknown allergen %q", raw))
			continue
		}
		seen[a] = true
		out = append(out, a)
	}
	return out
}

// ---------------------------------------------------------------- cursors

// A cursor is opaque to clients: base64url(JSON). It is bound to the list it came from
// (section / sort), so a cursor from price_asc is rejected for newest instead of silently skipping rows.
type catCursor struct {
	Scope string `json:"s"`
	K1    int64  `json:"a"`
	K2    int64  `json:"b"`
	ID    int64  `json:"i"`
}

func encodeCatCursor(scope string, k1, k2, id int64) *string {
	raw, _ := json.Marshal(catCursor{Scope: scope, K1: k1, K2: k2, ID: id})
	s := base64.RawURLEncoding.EncodeToString(raw)
	return &s
}

func (v *catValidator) cursor(raw, scope string) *model.CatalogueCursor {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if len(raw) > catCursorMaxEncoded {
		v.add("cursor", "invalid cursor")
		return nil
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	var c catCursor
	if err != nil || json.Unmarshal(b, &c) != nil || c.Scope != scope {
		v.add("cursor", "invalid cursor for this list (do not change filters or sort between pages)")
		return nil
	}
	return &model.CatalogueCursor{K1: c.K1, K2: c.K2, ID: c.ID}
}

// ---------------------------------------------------------------- presentation helpers

var catQueryReplacer = strings.NewReplacer("ي", "ی", "ك", "ک", "ـ", "", "\u200e", "", "\u200f", "")

// normalizeCatQuery unifies Arabic/Persian letter variants and collapses whitespace.
func normalizeCatQuery(s string) string {
	return strings.Join(strings.Fields(catQueryReplacer.Replace(s)), " ")
}

var tehranLoc = func() *time.Location {
	if loc, err := time.LoadLocation("Asia/Tehran"); err == nil {
		return loc
	}
	return time.FixedZone("Asia/Tehran", 3*3600+1800) // Iran has had no DST since 2022
}()

func availabilityBadge(qty int) string {
	return fmt.Sprintf("%s بسته موجود", toPersianDigits(fmt.Sprintf("%d", qty)))
}

func pickupWindowText(start, end, now time.Time) string {
	if start.IsZero() || end.IsZero() {
		return ""
	}
	s, e, n := start.In(tehranLoc), end.In(tehranLoc), now.In(tehranLoc)
	day := ""
	switch {
	case sameDay(s, n):
		day = "امروز، "
	case sameDay(s, n.AddDate(0, 0, 1)):
		day = "فردا، "
	}
	return fmt.Sprintf("%s%s-%s", day, toPersianDigits(s.Format("15:04")), toPersianDigits(e.Format("15:04")))
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func distanceText(m *int64) string {
	if m == nil {
		return ""
	}
	if *m < 1000 {
		return fmt.Sprintf("%s متر", toPersianDigits(fmt.Sprintf("%d", *m)))
	}
	return fmt.Sprintf("%s کیلومتر", toPersianDigits(fmt.Sprintf("%.1f", float64(*m)/1000)))
}
