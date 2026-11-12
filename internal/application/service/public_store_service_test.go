package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

type publicStoreRepositoryStub struct {
	store    *model.Store
	err      error
	publicID string
}

var _ repository.StoreRepository = (*publicStoreRepositoryStub)(nil)

func (r *publicStoreRepositoryStub) ListByUser(_ context.Context, _ int64) ([]*model.Store, error) {
	return nil, nil
}

func (r *publicStoreRepositoryStub) GetByPublicIDAndUser(_ context.Context, _ string, _ int64) (*model.Store, error) {
	return nil, nil
}

func (r *publicStoreRepositoryStub) GetPublicByPublicID(_ context.Context, publicID string) (*model.Store, error) {
	r.publicID = publicID
	return r.store, r.err
}

func (r *publicStoreRepositoryStub) Update(_ context.Context, _ *model.Store) error {
	return nil
}

type offerRepositoryStub struct {
	offers  []*model.Offer
	err     error
	storeID int64
}

var _ repository.OfferRepository = (*offerRepositoryStub)(nil)

func (r *offerRepositoryStub) ListAvailableByStoreID(_ context.Context, storeID int64) ([]*model.Offer, error) {
	r.storeID = storeID
	return r.offers, r.err
}

func TestPublicStoreServiceEligibleStoreWithOffers(t *testing.T) {
	district := "ولیعصر"
	cover := "https://example.com/cover.jpg"
	offerImg := "https://example.com/bread.jpg"
	dist := 0.6

	store := &model.Store{
		ID:                 10,
		PublicID:           "store-550e",
		VendorID:           999, // Private field
		VendorStatus:       "approved",
		IsActive:           true,
		AcceptsOrders:      true,
		Name:               "نان صبحگاهی",
		CategoryName:       "نانوایی",
		City:               "تهران",
		District:           &district,
		Biography:          "نانوایی محلی با نان‌های تازه و شیرینی‌های پخت روز.",
		AddressLine:        "تهران، خیابان ولیعصر، پلاک ۱۲",
		PickupInstructions: "تحویل از پیشخوان نانوایی، ورودی خیابان ولیعصر.",
		Location:           model.StoreLocation{Latitude: 35.6892, Longitude: 51.3890},
		CoverImageURL:      &cover,
		RatingAvg:          4.8, // Private
		TrustScore:         98,  // Private
		WorkingHours: []model.WorkingHour{
			{DayOfWeek: 0, DayNameFa: "شنبه", OpenTime: "08:00", CloseTime: "19:00", IsClosed: false},
			{DayOfWeek: 1, DayNameFa: "یکشنبه", OpenTime: "08:00", CloseTime: "19:00", IsClosed: false},
			{DayOfWeek: 4, DayNameFa: "چهارشنبه", OpenTime: "08:00", CloseTime: "19:00", IsClosed: false},
			{DayOfWeek: 5, DayNameFa: "پنج‌شنبه", IsClosed: true},
			{DayOfWeek: 6, DayNameFa: "جمعه", IsClosed: true},
		},
	}

	offers := []*model.Offer{
		{
			ID:                101,
			PublicID:          "off-1",
			StoreID:           10,
			Title:             "نان صبحگاهی",
			ImageURL:          &offerImg,
			OriginalPrice:     115000,
			Price:             40000,
			Currency:          "تومان",
			QuantityAvailable: 3,
			PickupWindowText:  "امروز، ۱۸:۰۰-۱۹:۰۰",
			DistanceKm:        &dist,
			IsActive:          true,
			CreatedAt:         time.Now(),
		},
	}

	storeRepo := &publicStoreRepositoryStub{store: store}
	offerRepo := &offerRepositoryStub{offers: offers}
	svc := NewPublicStoreService(storeRepo, offerRepo)

	res, err := svc.GetPublicStore(context.Background(), "store-550e")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Name != "نان صبحگاهی" {
		t.Fatalf("expected name نان صبحگاهی, got %s", res.Name)
	}
	if res.PickupAddress != "تهران، خیابان ولیعصر، پلاک ۱۲" {
		t.Fatalf("unexpected pickup address: %s", res.PickupAddress)
	}
	if res.PickupInstructions != "تحویل از پیشخوان نانوایی، ورودی خیابان ولیعصر." {
		t.Fatalf("unexpected pickup instructions: %s", res.PickupInstructions)
	}
	if res.HeaderSubtitle != "نانوایی · ولیعصر" {
		t.Fatalf("unexpected header subtitle: %s", res.HeaderSubtitle)
	}
	if !res.IsOpen || res.StatusBadge != "باز است" {
		t.Fatalf("expected open status: is_open=%v status_badge=%s", res.IsOpen, res.StatusBadge)
	}
	if len(res.Offers) != 1 {
		t.Fatalf("expected 1 offer, got %d", len(res.Offers))
	}
	if res.Offers[0].Price != 40000 || res.Offers[0].OriginalPrice != 115000 {
		t.Fatalf("unexpected offer pricing: %+v", res.Offers[0])
	}
	if res.Offers[0].QuantityAvailable != 3 {
		t.Fatalf("unexpected offer quantity: %d", res.Offers[0].QuantityAvailable)
	}
	if res.PickupSummary != "۳ موجود · تحویل امروز، ۱۸:۰۰-۱۹:۰۰" {
		t.Fatalf("unexpected pickup summary: %s", res.PickupSummary)
	}
	if len(res.WorkingHoursSchedule) == 0 {
		t.Fatal("expected working hours schedule to be populated")
	}
}

func TestPublicStoreServiceUnknownStore(t *testing.T) {
	storeRepo := &publicStoreRepositoryStub{store: nil}
	svc := NewPublicStoreService(storeRepo, nil)

	res, err := svc.GetPublicStore(context.Background(), "non-existent")
	if err == nil {
		t.Fatal("expected 404 error for unknown store")
	}
	if res != nil {
		t.Fatalf("expected nil response, got %+v", res)
	}
}

func TestPublicStoreServiceUnavailableStore(t *testing.T) {
	tests := []struct {
		name  string
		store *model.Store
	}{
		{
			name: "unapproved store",
			store: &model.Store{
				PublicID:     "store-pending",
				VendorStatus: "pending_review",
				IsActive:     true,
			},
		},
		{
			name: "rejected store",
			store: &model.Store{
				PublicID:     "store-rejected",
				VendorStatus: "rejected",
				IsActive:     true,
			},
		},
		{
			name: "inactive store",
			store: &model.Store{
				PublicID:     "store-inactive",
				VendorStatus: "approved",
				IsActive:     false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storeRepo := &publicStoreRepositoryStub{store: tt.store}
			svc := NewPublicStoreService(storeRepo, nil)

			res, err := svc.GetPublicStore(context.Background(), tt.store.PublicID)
			if err == nil {
				t.Fatal("expected 404 for unavailable store")
			}
			if res != nil {
				t.Fatalf("private store details should not be returned: %+v", res)
			}
		})
	}
}

func TestPublicStoreServiceNoAvailableOffers(t *testing.T) {
	store := &model.Store{
		ID:                 10,
		PublicID:           "store-no-offers",
		VendorStatus:       "approved",
		IsActive:           true,
		AcceptsOrders:      true,
		Name:               "نان صبحگاهی",
		AddressLine:        "تهران، خیابان ولیعصر",
		PickupInstructions: "تحویل از پیشخوان",
	}

	offers := []*model.Offer{
		{
			ID:                1,
			QuantityAvailable: 0, // Out of stock
			IsActive:          true,
		},
		{
			ID:                2,
			QuantityAvailable: 5,
			IsActive:          false, // Inactive
		},
	}

	storeRepo := &publicStoreRepositoryStub{store: store}
	offerRepo := &offerRepositoryStub{offers: offers}
	svc := NewPublicStoreService(storeRepo, offerRepo)

	res, err := svc.GetPublicStore(context.Background(), "store-no-offers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Name != "نان صبحگاهی" || res.PickupAddress != "تهران، خیابان ولیعصر" {
		t.Fatalf("store info should be returned: %+v", res)
	}

	if res.Offers == nil || len(res.Offers) != 0 {
		t.Fatalf("expected empty non-nil offers list, got %+v", res.Offers)
	}

	if res.PickupSummary != "" {
		t.Fatalf("expected empty pickup summary when no offers, got: %s", res.PickupSummary)
	}
}

func TestPublicStoreServiceStoreProfileUpdatesAppearConsistently(t *testing.T) {
	store := &model.Store{
		ID:                 10,
		PublicID:           "store-live",
		VendorStatus:       "approved",
		IsActive:           true,
		Name:               "Original Name",
		AddressLine:        "Original Address",
		PickupInstructions: "Original Instructions",
	}

	storeRepo := &publicStoreRepositoryStub{store: store}
	svc := NewPublicStoreService(storeRepo, nil)

	res1, err := svc.GetPublicStore(context.Background(), "store-live")
	if err != nil || res1.Name != "Original Name" {
		t.Fatalf("first lookup failed: %v", err)
	}

	// Vendor updates profile
	store.Name = "Updated Name"
	store.AddressLine = "Updated Address"
	store.PickupInstructions = "Updated Instructions"

	res2, err := svc.GetPublicStore(context.Background(), "store-live")
	if err != nil {
		t.Fatalf("second lookup failed: %v", err)
	}

	if res2.Name != "Updated Name" ||
		res2.PickupAddress != "Updated Address" ||
		res2.PickupInstructions != "Updated Instructions" {
		t.Fatalf("store profile updates were not reflected consistently: %+v", res2)
	}
}

func TestPublicStoreServiceRepositoryError(t *testing.T) {
	storeRepo := &publicStoreRepositoryStub{err: errors.New("db error")}
	svc := NewPublicStoreService(storeRepo, nil)

	_, err := svc.GetPublicStore(context.Background(), "store-1")
	if err == nil {
		t.Fatal("expected error on db failure")
	}
}
