package router_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	"github.com/tahedig/too-good-to-go/backend/internal/application/service"
	"github.com/tahedig/too-good-to-go/backend/internal/bootstrap"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/infrastructure/repository/memory"
	"github.com/tahedig/too-good-to-go/backend/internal/infrastructure/token"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/handler"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/middleware"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/router"
	"github.com/tahedig/too-good-to-go/backend/pkg/logger"
)

type adminDashEnv struct {
	r      *gin.Engine
	repo   *memory.AdminDashboardRepository
	tokens map[string]string
}

func newAdminDashEnv(t *testing.T) *adminDashEnv {
	t.Helper()
	log := logger.New("debug", &bytes.Buffer{})
	sessions := service.NewSessionService(
		memory.NewUserRepository(),
		token.NewJWTManager(rbacSecret, rbacIssuer, time.Hour),
		map[string]enum.Role{rbacVendorPhone: enum.RoleVendor, rbacAdminPhone: enum.RoleAdmin},
		log,
	)
	repo := memory.NewAdminDashboardRepository()
	repo.SetSnapshot(
		model.AdminDashboardTotals{
			Customers:             2,
			Vendors:               3,
			RegisteredStores:      5,
			ActiveStores:          1,
			ActiveOffers:          1,
			PendingApplications:   3,
			TotalReservations:     2,
			ActiveReservations:    2,
			CompletedReservations: 0,
			PlatformCommission:    200000,
		},
		model.AdminApplicationBreakdown{
			Pending:   3,
			Approved:  1,
			Rejected:  1,
			Suspended: 0,
		},
		model.AdminReservationBreakdown{
			Paid:   2,
			Active: 2,
			Total:  2,
		},
	)
	base := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	amt := int64(40000)
	repo.AddActivity(model.AdminActivityItem{
		Type: model.AdminActivityStoreApplication, EntityID: "30000000-0000-4000-8000-000000000001",
		Title: "نان صبحگاهی · شعبه شریعتی", ActorName: "آیدا کریمی", Status: "pending", OccurredAt: base,
	})
	repo.AddActivity(model.AdminActivityItem{
		Type: model.AdminActivityReservation, EntityID: "30000000-0000-4000-8000-000000000002",
		Title: "نان صبحگاهی", ActorName: "لیلا علیزاده", Status: "paid", Amount: &amt, OccurredAt: base.Add(time.Minute),
	})
	repo.AddActivity(model.AdminActivityItem{
		Type: model.AdminActivityStoreDecision, EntityID: "30000000-0000-4000-8000-000000000003",
		Title: "کافه رز", ActorName: "مدیر سامانه", Status: "approved", Action: "approve", OccurredAt: base.Add(-time.Minute),
	})

	r, err := router.New(
		&bootstrap.Config{CORSAllowedOrigins: []string{adminOrigin}},
		log,
		middleware.NewErrorHandler(log),
		middleware.NewAuthMiddleware(sessions),
		nil,
		router.Handlers{
			Health:         handler.NewHealthHandler(service.NewHealthService("test", time.Second, log)),
			Auth:           handler.NewAuthHandler(nil, nil),
			AdminDashboard: handler.NewAdminDashboardHandler(service.NewAdminDashboardService(repo)),
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	env := &adminDashEnv{r: r, repo: repo, tokens: map[string]string{}}
	for role, phone := range map[string]string{"customer": rbacCustomerPhone, "vendor": rbacVendorPhone, "admin": rbacAdminPhone} {
		res, err := sessions.Login(context.Background(), phone)
		if err != nil {
			t.Fatal(err)
		}
		env.tokens[role] = res.AccessToken
	}
	return env
}

func (e *adminDashEnv) call(path, role string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if role != "" {
		req.Header.Set("Authorization", "Bearer "+e.tokens[role])
	}
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	return w
}

func TestAdminDashboardRBAC(t *testing.T) {
	e := newAdminDashEnv(t)
	for role, want := range map[string]int{
		"":         http.StatusUnauthorized,
		"customer": http.StatusForbidden,
		"vendor":   http.StatusForbidden,
	} {
		if w := e.call("/admin/dashboard", role); w.Code != want {
			t.Errorf("GET /admin/dashboard as %q: got %d, want %d", role, w.Code, want)
		}
	}
}

func TestAdminDashboardMetricsAndActivityFeed(t *testing.T) {
	e := newAdminDashEnv(t)
	w := e.call("/admin/dashboard", "admin")
	if w.Code != http.StatusOK {
		t.Fatalf("admin dashboard: %d %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}

	var resp dto.AdminDashboardResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Totals.Customers != 2 || resp.Totals.Vendors != 3 || resp.Totals.RegisteredStores != 5 ||
		resp.Totals.ActiveStores != 1 || resp.Totals.ActiveOffers != 1 || resp.Totals.PendingApplications != 3 ||
		resp.Totals.TotalReservations != 2 || resp.Totals.ActiveReservations != 2 || resp.Totals.PlatformCommission != 200000 {
		t.Fatalf("totals: %+v", resp.Totals)
	}
	if resp.Applications != (dto.AdminApplicationBreakdownResponse{Pending: 3, Approved: 1, Rejected: 1, Suspended: 0}) {
		t.Fatalf("applications: %+v", resp.Applications)
	}
	if len(resp.RecentActivity) != 3 || resp.RecentActivity[0].Type != "reservation" ||
		resp.RecentActivity[1].Type != "store_application" || resp.RecentActivity[2].Type != "store_decision" {
		t.Fatalf("recent activity order: %+v", resp.RecentActivity)
	}

	// Filter activity feed by type + limit.
	w = e.call("/admin/dashboard?activity_type=store_application&activity_limit=1", "admin")
	var filtered dto.AdminDashboardResponse
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &filtered) != nil ||
		len(filtered.RecentActivity) != 1 || filtered.RecentActivity[0].Type != "store_application" {
		t.Fatalf("filtered activity: %d %s", w.Code, w.Body.String())
	}

	// Bad query parameters return 400.
	if w := e.call("/admin/dashboard?activity_limit=0&activity_type=invalid", "admin"); w.Code != http.StatusBadRequest {
		t.Fatalf("bad query: got %d, want 400", w.Code)
	}
}
