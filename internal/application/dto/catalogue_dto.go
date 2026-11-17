package dto

import "time"

// ---------- requests (parsed from the query string by the handler) ----------

// CatalogueQuery is the input of GET /offers and GET /stores/{id}/offers.
type CatalogueQuery struct {
	Q                string
	CategoryIDs      []int
	Kind             string
	PriceMin         *int64
	PriceMax         *int64
	PickupFrom       *time.Time
	PickupTo         *time.Time
	ExcludeAllergens []string
	Lat, Lng         *float64
	RadiusKm         *float64
	Sort             string
	Cursor           string
	Limit            *int
	UserID           *int64
}

// HomeQuery is the input of the three home sections.
type HomeQuery struct {
	Lat, Lng *float64
	RadiusKm *float64
	Cursor   string
	Limit    *int
	UserID   *int64
}

type MapQuery struct {
	Lat, Lng    *float64
	RadiusKm    *float64
	CategoryIDs []int
	Kind        string
	Limit       *int
	UserID      *int64
}

type FavoritesQuery struct {
	Lat, Lng *float64
	Cursor   string
	Limit    *int
}

// ---------- responses ----------

type CatalogueStoreRef struct {
	ID      string  `json:"id" example:"3f2b8f0e-6c1d-4c53-9a43-0d1d5c7a9e11"`
	Name    string  `json:"name" example:"نانوایی صبحگاهی"`
	LogoURL *string `json:"logo_url,omitempty" example:"https://example.com/logo.png"`
}

// CatalogueOfferCard is the one card shape used by every offer list (home, catalogue, favorites).
type CatalogueOfferCard struct {
	ID                string            `json:"id" example:"7d0c2a3e-1f5b-4a52-8c64-2f4e9b1a0c33"`
	Title             string            `json:"title" example:"بسته سورپرایز"`
	ImageURL          *string           `json:"image_url,omitempty"`
	Price             int64             `json:"price" example:"40000"`
	OriginalPrice     int64             `json:"original_price" example:"100000"`
	DiscountPercent   int               `json:"discount_percent" example:"60"`
	Currency          string            `json:"currency" example:"تومان"`
	QuantityAvailable int               `json:"quantity_available" example:"3"`
	AvailabilityBadge string            `json:"availability_badge" example:"۳ بسته موجود"`
	PickupStart       time.Time         `json:"pickup_start"`
	PickupEnd         time.Time         `json:"pickup_end"`
	PickupWindow      string            `json:"pickup_window" example:"امروز، ۱۸:۰۰-۱۹:۰۰"`
	CategorySlug      string            `json:"category_slug,omitempty" example:"bakery"`
	CategoryName      string            `json:"category_name,omitempty" example:"نانوایی"`
	DistanceM         *int64            `json:"distance_m,omitempty" example:"600"`
	DistanceText      string            `json:"distance_text,omitempty" example:"۰.۶ کیلومتر"`
	Store             CatalogueStoreRef `json:"store"`
	IsFavorite        bool              `json:"is_favorite" example:"false"`
}

// CatalogueOfferList is a keyset-paginated list. Send next_cursor back as ?cursor= to get the next page.
type CatalogueOfferList struct {
	Items      []CatalogueOfferCard `json:"items"`
	HasMore    bool                 `json:"has_more" example:"true"`
	NextCursor *string              `json:"next_cursor,omitempty" example:"eyJzIjoibmVhcmVzdCIsImsxIjo2MDB9"`
}

type HomeCategoryResponse struct {
	ID   int    `json:"id" example:"1"`
	Slug string `json:"slug" example:"bakery"`
	Name string `json:"name" example:"نانوایی"`
	Kind string `json:"kind" example:"bakery"`
}

type HomeCategoriesResponse struct {
	Items []HomeCategoryResponse `json:"items"`
}

type FilterOption struct {
	Value string `json:"value" example:"price_asc"`
	Label string `json:"label" example:"ارزان‌ترین"`
}

type FilterPriceRange struct {
	Min int64 `json:"min" example:"40000"`
	Max int64 `json:"max" example:"70000"`
}

type CatalogueFiltersResponse struct {
	Kinds       []FilterOption         `json:"kinds"`
	Categories  []HomeCategoryResponse `json:"categories"`
	Allergens   []FilterOption         `json:"allergens"`
	Price       FilterPriceRange       `json:"price"`
	SortOptions []FilterOption         `json:"sort_options"`
	OfferCount  int64                  `json:"offer_count" example:"22"`
}

type CatalogueMapOffer struct {
	ID                string                `json:"id"`
	Title             string                `json:"title"`
	ImageURL          *string               `json:"image_url,omitempty"`
	Price             int64                 `json:"price"`
	OriginalPrice     int64                 `json:"original_price"`
	DiscountPercent   int                   `json:"discount_percent"`
	Currency          string                `json:"currency"`
	QuantityAvailable int                   `json:"quantity_available"`
	PickupWindow      string                `json:"pickup_window"`
	CategorySlug      string                `json:"category_slug,omitempty"`
	DistanceM         *int64                `json:"distance_m,omitempty"`
	DistanceText      string                `json:"distance_text,omitempty"`
	Store             CatalogueStoreRef     `json:"store"`
	Location          StoreLocationResponse `json:"location"`
	IsFavorite        bool                  `json:"is_favorite"`
}

type CatalogueMapResponse struct {
	Center   StoreLocationResponse `json:"center"`
	RadiusKm float64               `json:"radius_km" example:"5"`
	Items    []CatalogueMapOffer   `json:"items"` // group by store.id on the client for one pin per store
}

type FavoriteOfferResponse struct {
	OfferID    string `json:"offer_id"`
	IsFavorite bool   `json:"is_favorite" example:"true"`
}
