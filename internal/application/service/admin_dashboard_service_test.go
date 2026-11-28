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

type stubAdminDashboardRepo struct {
	lastQuery repository.AdminDashboardQuery
	result    *model.AdminDashboard
	err       error
}

func (s *stubAdminDashboardRepo) GetDashboard(_ context.Context, q repository.AdminDashboardQuery) (*model.AdminDashboard, error) {
	s.lastQuery = q
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

func TestAdminDashboardServiceValidation(t *testing.T) {
	repo := &stubAdminDashboardRepo{result: &model.AdminDashboard{}}
	fixedNow := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	svc := &adminDashboardService{repo: repo, now: func() time.Time { return fixedNow }}

	badCases := []struct {
		name string
		in   serviceinterface.AdminDashboardInput
	}{
		{"limit zero", serviceinterface.AdminDashboardInput{ActivityLimit: "0"}},
		{"limit negative", serviceinterface.AdminDashboardInput{ActivityLimit: "-1"}},
		{"limit too large", serviceinterface.AdminDashboardInput{ActivityLimit: "51"}},
		{"limit non-numeric", serviceinterface.AdminDashboardInput{ActivityLimit: "ten"}},
		{"invalid activity_type", serviceinterface.AdminDashboardInput{ActivityType: "store_application,unknown"}},
	}
	for _, tc := range badCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.GetDashboard(context.Background(), tc.in)
			appErr, ok := err.(exception.AppError)
			if !ok || appErr.Code() != exception.CodeBadRequest {
				t.Fatalf("got %v, want BAD_REQUEST", err)
			}
		})
	}

	got, err := svc.GetDashboard(context.Background(), serviceinterface.AdminDashboardInput{
		ActivityLimit: "15",
		ActivityType:  "store_application, reservation, store_application",
	})
	if err != nil || got != repo.result {
		t.Fatalf("happy path: got=%v err=%v", got, err)
	}
	if repo.lastQuery.ActivityLimit != 15 || !repo.lastQuery.Now.Equal(fixedNow) ||
		len(repo.lastQuery.ActivityTypes) != 2 ||
		repo.lastQuery.ActivityTypes[0] != model.AdminActivityStoreApplication ||
		repo.lastQuery.ActivityTypes[1] != model.AdminActivityReservation {
		t.Fatalf("unexpected query forwarded to repo: %+v", repo.lastQuery)
	}

	if _, err := svc.GetDashboard(context.Background(), serviceinterface.AdminDashboardInput{}); err != nil {
		t.Fatalf("default query failed: %v", err)
	}
	if repo.lastQuery.ActivityLimit != DefaultAdminDashboardActivityLimit || len(repo.lastQuery.ActivityTypes) != 0 {
		t.Fatalf("unexpected defaults: %+v", repo.lastQuery)
	}
}

func TestAdminDashboardServiceInternalError(t *testing.T) {
	svc := NewAdminDashboardService(&stubAdminDashboardRepo{err: errors.New("db error")})
	_, err := svc.GetDashboard(context.Background(), serviceinterface.AdminDashboardInput{})
	appErr, ok := err.(exception.AppError)
	if !ok || appErr.Code() != exception.CodeInternalServer {
		t.Fatalf("got %v, want INTERNAL_SERVER_ERROR", err)
	}
}
