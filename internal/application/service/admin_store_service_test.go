package service_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/application/service"
	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/infrastructure/repository/memory"
)

const testAdminStoreID = "11111111-1111-4111-8111-111111111111"

func newTestAdminStoreService() (serviceinterface.AdminStoreService, *memory.AdminStoreRepository) {
	repo := memory.NewAdminStoreRepository()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return service.NewAdminStoreService(repo, log), repo
}

func seedTestStore(repo *memory.AdminStoreRepository) {
	open := "08:00"
	closeT := "19:00"
	district := "ولیعصر"
	repo.AddStore(model.AdminStoreDetail{
		ID:           testAdminStoreID,
		Name:         "نان صبحگاهی",
		OwnerName:    "آیدا کریمی",
		Category:     "نانوایی",
		Phone:        "09125550120",
		Biography:    "نانوایی سنگک",
		IsActive:     true,
		VendorStatus: enum.StoreStatusApproved,
		CreatedAt:    time.Date(2026, 10, 17, 8, 0, 0, 0, time.UTC),
		Location: model.AdminStoreLocation{
			Latitude:    35.7219,
			Longitude:   51.3347,
			City:        "تهران",
			District:    &district,
			AddressLine: "خیابان ولیعصر، پلاک ۱۲",
		},
		WorkingHours: []model.AdminStoreWorkingHour{
			{DayOfWeek: 0, OpenTime: &open, CloseTime: &closeT, IsClosed: false},
			{DayOfWeek: 5, IsClosed: true},
		},
	})
}

func assertAppErrCode(t *testing.T, err error, want string) {
	t.Helper()
	var appErr exception.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError(%s), got %v", want, err)
	}
	if appErr.Code() != want {
		t.Fatalf("expected code %s, got %s", want, appErr.Code())
	}
}

func TestAdminStoreService_GetAndToggleStatus(t *testing.T) {
	svc, repo := newTestAdminStoreService()
	seedTestStore(repo)
	ctx := context.Background()

	got, err := svc.GetStore(ctx, testAdminStoreID)
	if err != nil {
		t.Fatalf("GetStore error: %v", err)
	}
	if !got.IsActive || got.Name != "نان صبحگاهی" || len(got.WorkingHours) != 2 {
		t.Fatalf("unexpected store: %+v", got)
	}

	inactive := false
	updated, err := svc.UpdateStoreStatus(ctx, testAdminStoreID, &inactive)
	if err != nil {
		t.Fatalf("UpdateStoreStatus(false) error: %v", err)
	}
	if updated.IsActive {
		t.Fatalf("expected IsActive=false")
	}

	active := true
	updated, err = svc.UpdateStoreStatus(ctx, testAdminStoreID, &active)
	if err != nil {
		t.Fatalf("UpdateStoreStatus(true) error: %v", err)
	}
	if !updated.IsActive {
		t.Fatalf("expected IsActive=true")
	}
}

func TestAdminStoreService_Errors(t *testing.T) {
	svc, _ := newTestAdminStoreService()
	ctx := context.Background()

	_, err := svc.GetStore(ctx, "not-a-uuid")
	assertAppErrCode(t, err, exception.CodeNotFound)

	_, err = svc.GetStore(ctx, testAdminStoreID)
	assertAppErrCode(t, err, exception.CodeNotFound)

	_, err = svc.UpdateStoreStatus(ctx, testAdminStoreID, nil)
	assertAppErrCode(t, err, exception.CodeBadRequest)
}

func TestAdminStoreService_Subscription(t *testing.T) {
	svc, repo := newTestAdminStoreService()
	seedTestStore(repo)
	ctx := context.Background()

	_, err := svc.GetStoreSubscription(ctx, testAdminStoreID)
	assertAppErrCode(t, err, exception.CodeNotFound)

	upgrade := "حرفه‌ای"
	repo.AddSubscription(model.AdminStoreSubscription{
		ID:             "22222222-2222-4222-8222-222222222222",
		StoreID:        testAdminStoreID,
		Status:         "pending_review",
		CurrentPlan:    "پایه - رایگان",
		CommissionRate: 10,
		PeriodStart:    time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC),
		PeriodEnd:      time.Date(2026, 10, 22, 23, 59, 59, 0, time.UTC),
		RemainingDays:  3,
		UpgradeRequest: &upgrade,
	})

	sub, err := svc.GetStoreSubscription(ctx, testAdminStoreID)
	if err != nil {
		t.Fatalf("GetStoreSubscription error: %v", err)
	}
	if sub.CurrentPlan != "پایه - رایگان" || sub.RemainingDays != 3 || sub.UpgradeRequest == nil || *sub.UpgradeRequest != "حرفه‌ای" {
		t.Fatalf("unexpected subscription: %+v", sub)
	}
}

func TestAdminStoreService_Offers(t *testing.T) {
	svc, repo := newTestAdminStoreService()
	seedTestStore(repo)
	ctx := context.Background()

	offerID := "33333333-3333-4333-8333-333333333333"
	repo.AddOffer(model.AdminOffer{
		ID:                offerID,
		StoreID:           testAdminStoreID,
		StoreName:         "نان صبحگاهی",
		OwnerName:         "آیدا کریمی",
		Category:          "نانوایی",
		Title:             "بسته غافلگیری نانوایی",
		Description:       "نان تازه پایان روز",
		OfferType:         "surprise_bag",
		OriginalPrice:     115000,
		DiscountPrice:     40000,
		QuantityTotal:     5,
		QuantityAvailable: 3,
		Status:            "active",
		IsActive:          true,
		PickupStart:       time.Date(2026, 10, 18, 15, 0, 0, 0, time.UTC),
		PickupEnd:         time.Date(2026, 10, 18, 19, 0, 0, 0, time.UTC),
	})

	offers, err := svc.ListStoreOffers(ctx, testAdminStoreID)
	if err != nil || len(offers) != 1 {
		t.Fatalf("ListStoreOffers unexpected: %v, len=%d", err, len(offers))
	}

	got, err := svc.GetOffer(ctx, offerID)
	if err != nil || got.DiscountPrice != 40000 {
		t.Fatalf("GetOffer unexpected: %v, %+v", err, got)
	}

	inactive := false
	updated, err := svc.UpdateOfferStatus(ctx, offerID, &inactive)
	if err != nil || updated.IsActive || updated.Status != "cancelled" {
		t.Fatalf("UpdateOfferStatus unexpected: %v, %+v", err, updated)
	}

	_, err = svc.UpdateOfferStatus(ctx, offerID, nil)
	assertAppErrCode(t, err, exception.CodeBadRequest)
}
