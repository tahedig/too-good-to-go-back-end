package service

import (
	"context"
	"errors"
	"testing"
	"time"

	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

type stubVendorDashboardRepo struct {
	lastQuery repository.VendorDashboardQuery
	result    *model.VendorDashboard
	err       error
}

func (s *stubVendorDashboardRepo) GetDashboard(_ context.Context, q repository.VendorDashboardQuery) (*model.VendorDashboard, error) {
	s.lastQuery = q
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

func TestVendorDashboardServiceValidation(t *testing.T) {
	repo := &stubVendorDashboardRepo{result: &model.VendorDashboard{}}
	fixedNow := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	svc := &vendorDashboardService{repo: repo, now: func() time.Time { return fixedNow }}

	if _, err := svc.GetDashboard(context.Background(), "", serviceinterface.VendorDashboardInput{}); err == nil {
		t.Fatal("expected unauthorized for empty vendor user ID")
	} else if appErr, ok := err.(exception.AppError); !ok || appErr.Code() != exception.CodeUnauthorized {
		t.Fatalf("got %v, want UNAUTHORIZED", err)
	}

	badCases := []struct {
		name string
		in   serviceinterface.VendorDashboardInput
	}{
		{"invalid store_id", serviceinterface.VendorDashboardInput{StoreID: "not-a-uuid"}},
		{"recent_limit zero", serviceinterface.VendorDashboardInput{RecentLimit: "0"}},
		{"recent_limit negative", serviceinterface.VendorDashboardInput{RecentLimit: "-5"}},
		{"recent_limit too large", serviceinterface.VendorDashboardInput{RecentLimit: "51"}},
		{"recent_limit non-numeric", serviceinterface.VendorDashboardInput{RecentLimit: "abc"}},
	}
	for _, tc := range badCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.GetDashboard(context.Background(), "00000000-0000-4000-8000-000000000001", tc.in)
			appErr, ok := err.(exception.AppError)
			if !ok || appErr.Code() != exception.CodeBadRequest {
				t.Fatalf("got %v, want BAD_REQUEST", err)
			}
		})
	}

	storeID := "550E8400-E29B-41D4-A716-446655440000"
	got, err := svc.GetDashboard(context.Background(), "00000000-0000-4000-8000-000000000001", serviceinterface.VendorDashboardInput{
		StoreID:     " " + storeID + " ",
		RecentLimit: "25",
	})
	if err != nil || got != repo.result {
		t.Fatalf("happy path: got=%v err=%v", got, err)
	}
	if repo.lastQuery.StoreID != "550e8400-e29b-41d4-a716-446655440000" ||
		repo.lastQuery.RecentLimit != 25 || !repo.lastQuery.Now.Equal(fixedNow) {
		t.Fatalf("unexpected query forwarded to repo: %+v", repo.lastQuery)
	}

	if _, err := svc.GetDashboard(context.Background(), "00000000-0000-4000-8000-000000000001", serviceinterface.VendorDashboardInput{}); err != nil {
		t.Fatalf("default query failed: %v", err)
	}
	if repo.lastQuery.StoreID != "" || repo.lastQuery.RecentLimit != DefaultVendorDashboardRecentLimit {
		t.Fatalf("unexpected defaults: %+v", repo.lastQuery)
	}
}

func TestVendorDashboardServiceErrorMapping(t *testing.T) {
	ctx := context.Background()
	vendorID := "00000000-0000-4000-8000-000000000001"

	notFoundSvc := NewVendorDashboardService(&stubVendorDashboardRepo{err: repository.ErrStoreNotFound})
	if _, err := notFoundSvc.GetDashboard(ctx, vendorID, serviceinterface.VendorDashboardInput{}); err == nil {
		t.Fatal("expected not found")
	} else if appErr, ok := err.(exception.AppError); !ok || appErr.Code() != exception.CodeNotFound {
		t.Fatalf("got %v, want NOT_FOUND", err)
	}

	internalSvc := NewVendorDashboardService(&stubVendorDashboardRepo{err: errors.New("db down")})
	if _, err := internalSvc.GetDashboard(ctx, vendorID, serviceinterface.VendorDashboardInput{}); err == nil {
		t.Fatal("expected internal server error")
	} else if appErr, ok := err.(exception.AppError); !ok || appErr.Code() != exception.CodeInternalServer {
		t.Fatalf("got %v, want INTERNAL_SERVER_ERROR", err)
	}
}
