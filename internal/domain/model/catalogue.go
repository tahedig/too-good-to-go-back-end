package model

import "time"

// CatalogueCursor is the keyset position of the last row a client has seen.
// Catalogue / favorites use K1 only (K2 = 0); home sections use K1 and K2.
type CatalogueCursor struct {
	K1, K2, ID int64
}

// CatalogueSearch is the validated input of the catalogue query (scripts/catalogue/sql/catalogue_search.sql).
type CatalogueSearch struct {
	Query            string // raw text; the repository escapes LIKE wildcards
	CategoryIDs      []int16
	Kind             string
	PriceMin         *int64
	PriceMax         *int64
	PickupFrom       *time.Time
	PickupTo         *time.Time
	ExcludeAllergens []string
	StorePublicID    string
	Lat, Lng         *float64
	RadiusKm         *float64
	Sort             string // nearest | price_asc | price_desc | newest
	Cursor           *CatalogueCursor
	UserID           *int64
	Limit            int // the service passes page size + 1
}

type CatalogueFavoritesSearch struct {
	UserID   int64
	Lat, Lng *float64
	Cursor   *CatalogueCursor
	Limit    int
}

type CatalogueMapSearch struct {
	Lat, Lng    float64
	RadiusKm    float64
	CategoryIDs []int16
	Kind        string
	UserID      *int64
	Limit       int
}

type HomeSection string

const (
	HomeSectionToday         HomeSection = "today"
	HomeSectionPopularNearby HomeSection = "popular_nearby"
	HomeSectionLastChance    HomeSection = "last_chance"
)

type HomeSearch struct {
	Lat, Lng *float64
	RadiusKm float64
	Cursor   *CatalogueCursor
	UserID   *int64
	Limit    int
}

// CatalogueOffer is one row of the catalogue and the favorites list.
type CatalogueOffer struct {
	PublicID          string
	Title             string
	ImageURL          *string
	Price             int64
	OriginalPrice     int64
	DiscountPercent   int
	Currency          string
	QuantityAvailable int
	PickupStart       time.Time
	PickupEnd         time.Time
	CategorySlug      string
	CategoryName      string
	StorePublicID     string
	StoreName         string
	StoreLogoURL      *string
	DistanceM         *int64
	SortKey           int64
	ID                int64
	IsFavorite        bool
}

// HomeOffer is one row of a home section.
type HomeOffer struct {
	PublicID          string
	Title             string
	ImageURL          *string
	Price             int64
	OriginalPrice     int64
	DiscountPercent   int
	Currency          string
	QuantityAvailable int
	PickupStart       time.Time
	PickupEnd         time.Time
	StorePublicID     string
	StoreName         string
	DistanceM         *int64
	K1, K2, ID        int64
	IsFavorite        bool
}

// MapOffer is one pin of the map.
type MapOffer struct {
	PublicID          string
	Title             string
	ImageURL          *string
	Price             int64
	OriginalPrice     int64
	DiscountPercent   int
	Currency          string
	QuantityAvailable int
	PickupStart       time.Time
	PickupEnd         time.Time
	CategorySlug      string
	StorePublicID     string
	StoreName         string
	StoreLat          float64
	StoreLng          float64
	DistanceM         *int64
	IsFavorite        bool
}

type CatalogueCategory struct {
	ID     int
	Slug   string
	NameFA string
	Kind   string
}

type CatalogueAllergen struct {
	Code   string
	NameFA string
}

type CatalogueFilterData struct {
	Allergens  []CatalogueAllergen
	PriceMin   int64
	PriceMax   int64
	OfferCount int64
}
