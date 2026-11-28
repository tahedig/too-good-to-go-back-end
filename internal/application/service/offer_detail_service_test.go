package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
)

func TestOfferDetailSuccessAndValidation(t *testing.T) {
	offerID := "7d0c2a3e-1f5b-4a52-8c64-2f4e9b1a0c33"
	storeID := "3f2b8f0e-6c1d-4c53-9a43-0d1d5c7a9e11"
	pickupStart := time.Date(2026, 11, 23, 14, 30, 0, 0, time.UTC)
	pickupEnd := pickupStart.Add(time.Hour)
	dist := int64(600)

	repo := &fakeCatalogueRepo{
		detail: &model.CatalogueOfferDetail{
			ID:                      1,
			PublicID:                offerID,
			Title:                   "بسته غافلگیری نانوایی",
			Description:             "ترکیبی از نان‌ها و شیرینی‌های باقی‌مانده پخت امروز.",
			Price:                   40000,
			OriginalPrice:           115000,
			DiscountPercent:         65,
			Currency:                "تومان",
			QuantityAvailable:       3,
			PickupStart:             pickupStart,
			PickupEnd:               pickupEnd,
			CategorySlug:            "bakery",
			CategoryName:            "نانوایی",
			Allergens:               []model.CatalogueAllergen{{Code: "gluten", NameFA: "گلوتن"}},
			AllergensDeclared:       true,
			StorePublicID:           storeID,
			StoreName:               "نان صبحگاهی",
			StoreCity:               "تهران",
			StoreAddressLine:        "خیابان ولیعصر، پلاک ۱۲",
			StorePickupInstructions: "در بازه تعیین‌شده از پیشخوان تحویل بگیرید.",
			StoreLat:                35.70,
			StoreLng:                51.41,
			DistanceM:               &dist,
			IsFavorite:              true,
		},
	}
	svc := NewCatalogueService(repo)

	res, err := svc.OfferDetail(context.Background(), offerID, dto.OfferDetailQuery{
		Lat: catF64(35.70),
		Lng: catF64(51.41),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ID != offerID || res.Store.ID != storeID {
		t.Fatalf("unexpected IDs: %+v", res)
	}
	if res.Store.Address != "تهران، خیابان ولیعصر، پلاک ۱۲" {
		t.Errorf("got address %q", res.Store.Address)
	}
	if !res.CancellationDeadline.Equal(pickupStart.Add(-2 * time.Hour)) {
		t.Errorf("got cancellation deadline %v, want %v", res.CancellationDeadline, pickupStart.Add(-2*time.Hour))
	}
	if !res.AllergensDeclared || len(res.Allergens) != 1 || res.Allergens[0].Value != "gluten" {
		t.Errorf("unexpected allergens: %+v", res.Allergens)
	}
	if !res.IsFavorite || res.DistanceM == nil || *res.DistanceM != 600 {
		t.Errorf("unexpected distance/favorite: %+v", res)
	}

	// Invalid UUID -> 400
	if _, err := svc.OfferDetail(context.Background(), "not-a-uuid", dto.OfferDetailQuery{}); err == nil {
		t.Fatal("expected error for invalid UUID")
	} else if appErr, ok := err.(exception.AppError); !ok || appErr.StatusCode() != http.StatusBadRequest {
		t.Fatalf("want 400, got %v", err)
	}

	// Partial lat/lng -> 400
	if _, err := svc.OfferDetail(context.Background(), offerID, dto.OfferDetailQuery{Lat: catF64(35.7)}); err == nil {
		t.Fatal("expected error for missing lng")
	} else if appErr, ok := err.(exception.AppError); !ok || appErr.StatusCode() != http.StatusBadRequest {
		t.Fatalf("want 400, got %v", err)
	}

	// Explicit cancellation_deadline and allergens_declared=false override defaults
	customDeadline := pickupStart.Add(-30 * time.Minute)
	repo.detail.CancellationDeadline = &customDeadline
	repo.detail.AllergensDeclared = false
	res2, err := svc.OfferDetail(context.Background(), offerID, dto.OfferDetailQuery{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res2.CancellationDeadline.Equal(customDeadline) {
		t.Errorf("got cancellation deadline %v, want %v", res2.CancellationDeadline, customDeadline)
	}
	if res2.AllergensDeclared {
		t.Errorf("got allergens_declared=true, want false when offer has allergens_declared=false")
	}

	// Not found / ineligible -> 404
	repo.detail = nil
	if _, err := svc.OfferDetail(context.Background(), offerID, dto.OfferDetailQuery{}); err == nil {
		t.Fatal("expected 404 for missing/ineligible offer")
	} else if appErr, ok := err.(exception.AppError); !ok || appErr.StatusCode() != http.StatusNotFound {
		t.Fatalf("want 404, got %v", err)
	}
}
