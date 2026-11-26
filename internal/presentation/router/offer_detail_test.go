package router_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	"github.com/tahedig/too-good-to-go/backend/internal/application/service"
	"github.com/tahedig/too-good-to-go/backend/internal/bootstrap"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/handler"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/middleware"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/router"
	"github.com/tahedig/too-good-to-go/backend/pkg/logger"
)

type stubOfferDetailCatalogueRepo struct {
	detail *model.CatalogueOfferDetail
}

func (s *stubOfferDetailCatalogueRepo) Categories(context.Context) ([]model.CatalogueCategory, error) {
	return nil, nil
}
func (s *stubOfferDetailCatalogueRepo) Search(context.Context, model.CatalogueSearch) ([]model.CatalogueOffer, error) {
	return nil, nil
}
func (s *stubOfferDetailCatalogueRepo) Favorites(context.Context, model.CatalogueFavoritesSearch) ([]model.CatalogueOffer, error) {
	return nil, nil
}
func (s *stubOfferDetailCatalogueRepo) Home(context.Context, model.HomeSection, model.HomeSearch) ([]model.HomeOffer, error) {
	return nil, nil
}
func (s *stubOfferDetailCatalogueRepo) Map(context.Context, model.CatalogueMapSearch) ([]model.MapOffer, error) {
	return nil, nil
}
func (s *stubOfferDetailCatalogueRepo) FilterData(context.Context) (model.CatalogueFilterData, error) {
	return model.CatalogueFilterData{}, nil
}
func (s *stubOfferDetailCatalogueRepo) OfferDetail(_ context.Context, id string, _, _ *float64, _ *int64) (*model.CatalogueOfferDetail, error) {
	if s.detail != nil && s.detail.PublicID == id {
		return s.detail, nil
	}
	return nil, nil
}
func (s *stubOfferDetailCatalogueRepo) AddFavorite(context.Context, int64, string) (bool, error) {
	return false, nil
}
func (s *stubOfferDetailCatalogueRepo) RemoveFavorite(context.Context, int64, string) error {
	return nil
}
func (s *stubOfferDetailCatalogueRepo) UserExists(context.Context, int64) (bool, error) {
	return true, nil
}

func TestOfferDetailRoute(t *testing.T) {
	log := logger.New("debug", &bytes.Buffer{})
	catRepo := &stubOfferDetailCatalogueRepo{}
	r, err := router.New(&bootstrap.Config{}, log, middleware.NewErrorHandler(log), nil, nil, router.Handlers{
		Health:    handler.NewHealthHandler(service.NewHealthService("test", time.Second, log)),
		Auth:      handler.NewAuthHandler(nil, nil),
		Catalogue: handler.NewCatalogueHandler(service.NewCatalogueService(catRepo)),
	})
	if err != nil {
		t.Fatal(err)
	}

	offerID := "7d0c2a3e-1f5b-4a52-8c64-2f4e9b1a0c33"
	pickupStart := time.Now().UTC().Add(3 * time.Hour)
	catRepo.detail = &model.CatalogueOfferDetail{
		ID:                      1,
		PublicID:                offerID,
		Title:                   "بسته غافلگیری نانوایی",
		Description:             "ترکیبی از نان‌های تازه",
		Price:                   40000,
		OriginalPrice:           115000,
		DiscountPercent:         65,
		Currency:                "تومان",
		QuantityAvailable:       3,
		PickupStart:             pickupStart,
		PickupEnd:               pickupStart.Add(time.Hour),
		CategorySlug:            "bakery",
		CategoryName:            "نانوایی",
		StorePublicID:           "550e8400-e29b-41d4-a716-446655440000",
		StoreName:               "نان صبحگاهی",
		StoreCity:               "تهران",
		StoreAddressLine:        "خیابان ولیعصر، پلاک ۱۲",
		StorePickupInstructions: "از پیشخوان تحویل بگیرید.",
		StoreLat:                35.70,
		StoreLng:                51.41,
	}

	// Happy path
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/offers/"+offerID+"?lat=35.7&lng=51.41", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /offers/:id got %d: %s", w.Code, w.Body.String())
	}
	var detail dto.OfferDetailResponse
	if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if detail.ID != offerID || detail.QuantityAvailable != 3 || detail.Store.Name != "نان صبحگاهی" {
		t.Errorf("unexpected detail: %+v", detail)
	}

	// Static route /offers/filters still works
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/offers/filters", nil))
	if w.Code != http.StatusOK {
		t.Errorf("GET /offers/filters got %d, want 200", w.Code)
	}

	// Unknown / ineligible offer -> 404
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/offers/88888888-8888-4888-8888-888888888888", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("unknown offer got %d, want 404", w.Code)
	}

	// Invalid UUID -> 400
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/offers/not-a-uuid", nil))
	if w.Code != http.StatusBadRequest {
		t.Errorf("invalid UUID got %d, want 400", w.Code)
	}
}
