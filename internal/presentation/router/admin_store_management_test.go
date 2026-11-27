package router_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	"github.com/tahedig/too-good-to-go/backend/internal/application/service"
	"github.com/tahedig/too-good-to-go/backend/internal/bootstrap"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/handler"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/middleware"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/router"
	"github.com/tahedig/too-good-to-go/backend/pkg/logger"
)

type adminManagementRepositoryStub struct {
	store  *model.Store
	filter repository.AdminStoreFilter
}

func (r *adminManagementRepositoryStub) ListAdmin(_ context.Context, filter repository.AdminStoreFilter) (*repository.AdminStorePage, error) {
	r.filter = filter
	return &repository.AdminStorePage{Items: []*model.Store{r.store}, Total: 1}, nil
}

func (r *adminManagementRepositoryStub) GetByPublicID(_ context.Context, _ string) (*model.Store, error) {
	return r.store, nil
}

func (r *adminManagementRepositoryStub) CategoryIsActive(_ context.Context, _ int16) (bool, error) {
	return true, nil
}

func (r *adminManagementRepositoryStub) Update(_ context.Context, store *model.Store) error {
	r.store = store
	return nil
}

func newAdminStoreManagementRouter(t *testing.T, repo *adminManagementRepositoryStub) *rbacEnv {
	t.Helper()
	base := newRBACEnv(t)
	log := logger.New("debug", &bytes.Buffer{})
	r, err := router.New(
		&bootstrap.Config{},
		log,
		middleware.NewErrorHandler(log),
		middleware.NewAuthMiddleware(base.sessions),
		nil,
		router.Handlers{
			Health: handler.NewHealthHandler(service.NewHealthService("test", time.Second, log)),
			Auth:   handler.NewAuthHandler(nil, nil),
			AdminStoreManagement: handler.NewAdminStoreManagementHandler(
				service.NewAdminStoreManagementService(repo),
			),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	base.r = r
	return base
}

func TestAdminStoreManagementRoutesRequireAdminAndParseFilters(t *testing.T) {
	repo := &adminManagementRepositoryStub{store: &model.Store{
		ID: 1, PublicID: adminStoreTestID, VendorID: 8, VendorStatus: "approved",
		Name: "نان تازه", OwnerName: "آیدا", CategoryName: "نانوایی", City: "تهران",
		Biography: "نان روز", AddressLine: "ولیعصر", IsActive: true, AcceptsOrders: true,
		CreatedAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
	}}
	e := newAdminStoreManagementRouter(t, repo)

	for _, tc := range []struct {
		name string
		auth string
		code int
	}{
		{name: "unauthenticated", code: http.StatusUnauthorized},
		{name: "customer", auth: "Bearer " + e.login(t, rbacCustomerPhone).AccessToken, code: http.StatusForbidden},
		{name: "vendor", auth: "Bearer " + e.login(t, rbacVendorPhone).AccessToken, code: http.StatusForbidden},
	} {
		w := e.get("/admin/stores", tc.auth)
		if w.Code != tc.code {
			t.Errorf("%s: GET /admin/stores got %d %s, want %d", tc.name, w.Code, w.Body.String(), tc.code)
		}
	}

	admin := e.login(t, rbacAdminPhone)
	w := e.get("/admin/stores?search=%D9%86%D8%A7%D9%86&status=approved&category_id=3&city=%D8%AA%D9%87%D8%B1%D8%A7%D9%86&accepts_orders=false&include_deleted=true&sort=oldest&limit=5",
		"Bearer "+admin.AccessToken)
	if w.Code != http.StatusOK {
		t.Fatalf("admin GET /admin/stores: %d %s", w.Code, w.Body.String())
	}
	var response dto.AdminManagedStoreListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || len(response.Items) != 1 {
		t.Fatalf("decode admin store list: %v, %s", err, w.Body.String())
	}
	if repo.filter.Search != "نان" || repo.filter.Status != "approved" || repo.filter.CategoryID == nil ||
		*repo.filter.CategoryID != 3 || repo.filter.City != "تهران" || repo.filter.AcceptsOrders == nil ||
		*repo.filter.AcceptsOrders || !repo.filter.IncludeDeleted || repo.filter.Sort != "oldest" || repo.filter.Limit != 6 {
		t.Fatalf("filters did not reach repository: %+v", repo.filter)
	}

	w = e.get("/admin/stores/"+adminStoreTestID, "Bearer "+admin.AccessToken)
	if w.Code != http.StatusOK {
		t.Fatalf("admin GET /admin/stores/:id: %d %s", w.Code, w.Body.String())
	}
	var detail dto.AdminManagedStoreResponse
	if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil ||
		detail.ID != adminStoreTestID || detail.Name != "نان تازه" || detail.Status != "approved" {
		t.Fatalf("unexpected store detail: %+v, %v", detail, err)
	}

	for _, path := range []string{
		"/admin/stores?limit=invalid",
		"/admin/stores?limit=",
		"/admin/stores?accepts_orders=invalid",
		"/admin/stores?accepts_orders=1",
		"/admin/stores?include_deleted=invalid",
		"/admin/stores?category_id=",
		"/admin/stores?status=unknown",
	} {
		w = e.get(path, "Bearer "+admin.AccessToken)
		if w.Code != http.StatusBadRequest || decodeError(t, w).Code != exception.CodeBadRequest {
			t.Errorf("invalid query %q: got %d %s, want standard 400", path, w.Code, w.Body.String())
		}
	}

	w = e.get("/admin/stores/not-a-uuid", "Bearer "+admin.AccessToken)
	if w.Code != http.StatusBadRequest || decodeError(t, w).Code != exception.CodeBadRequest {
		t.Fatalf("malformed public id: got %d %s, want standard 400", w.Code, w.Body.String())
	}
}

func TestAdminStoreManagementUpdateIgnoresProtectedFields(t *testing.T) {
	repo := &adminManagementRepositoryStub{store: &model.Store{
		ID: 1, PublicID: adminStoreTestID, VendorID: 42, VendorStatus: "approved",
		OwnerName: "مالک اصلی", Name: "نام قبلی", Biography: "معرفی", City: "تهران",
		AddressLine: "نشانی", IsActive: true, AcceptsOrders: true,
		RatingAvg: 4.5, RatingCount: 12, TrustScore: 90, MealsSaved: 30,
	}}
	e := newAdminStoreManagementRouter(t, repo)
	admin := e.login(t, rbacAdminPhone)
	body := `{"name":"نام جدید","accepts_orders":false,"public_id":"00000000-0000-4000-8000-000000000001","vendor_id":999,"owner_name":"متجاوز","status":"rejected","approved_by":999,"approved_at":"2020-01-01T00:00:00Z","rejection_reason":"bad","rating_avg":0,"rating_count":0,"trust_score":0,"meals_saved":0,"created_at":"2000-01-01T00:00:00Z"}`
	w := e.patch("/admin/stores/"+adminStoreTestID, "Bearer "+admin.AccessToken, body)
	if w.Code != http.StatusOK {
		t.Fatalf("PATCH /admin/stores/:id: %d %s", w.Code, w.Body.String())
	}
	if repo.store.Name != "نام جدید" || repo.store.AcceptsOrders {
		t.Fatalf("permitted update was not applied: %+v", repo.store)
	}
	if repo.store.PublicID != adminStoreTestID || repo.store.VendorID != 42 || repo.store.OwnerName != "مالک اصلی" ||
		repo.store.VendorStatus != "approved" || !repo.store.IsActive || repo.store.RatingAvg != 4.5 ||
		repo.store.RatingCount != 12 || repo.store.TrustScore != 90 || repo.store.MealsSaved != 30 {
		t.Fatalf("protected fields changed from request: %+v", repo.store)
	}

	var result dto.AdminManagedStoreResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if result.Status != "approved" || result.AcceptsOrders || result.IsActive {
		t.Fatalf("approval status and order acceptance were conflated: %+v", result)
	}
}
