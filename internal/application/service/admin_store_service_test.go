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
