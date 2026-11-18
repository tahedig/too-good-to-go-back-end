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

const adminStoreTestID = "11111111-1111-4111-8111-111111111111"

type adminStoreEnv struct {
	r      *gin.Engine
	repo   *memory.AdminStoreRepository
	tokens map[string]string
}

func newAdminStoreEnv(t *testing.T) *adminStoreEnv {
	t.Helper()
	log := logger.New("debug", &bytes.Buffer{})
	sessions := service.NewSessionService(
		memory.NewUserRepository(),
		token.NewJWTManager(rbacSecret, rbacIssuer, time.Hour),
		map[string]enum.Role{rbacVendorPhone: enum.RoleVendor, rbacAdminPhone: enum.RoleAdmin},
		log,
	)
	repo := memory.NewAdminStoreRepository()
	svc := service.NewAdminStoreService(repo, log)
	r, err := router.New(
		&bootstrap.Config{CORSAllowedOrigins: []string{adminOrigin}},
		log,
		middleware.NewErrorHandler(log),
		middleware.NewAuthMiddleware(sessions),
		nil,
		router.Handlers{
			Health:     handler.NewHealthHandler(service.NewHealthService("test", time.Second, log)),
			Auth:       handler.NewAuthHandler(nil, nil),
			AdminStore: handler.NewAdminStoreHandler(svc),
		},
	)
	if err != nil {
		t.Fatalf("router.New: %v", err)
	}
	tokens := make(map[string]string, 3)
	for role, phone := range map[string]string{
		"customer": rbacCustomerPhone,
		"vendor":   rbacVendorPhone,
		"admin":    rbacAdminPhone,
	} {
		pair, err := sessions.Login(context.Background(), phone)
		if err != nil {
			t.Fatalf("Login(%s): %v", role, err)
		}
		tokens[role] = pair.AccessToken
	}
	return &adminStoreEnv{r: r, repo: repo, tokens: tokens}
}

func (e *adminStoreEnv) seedStore() {
	open := "08:00"
	closeT := "19:00"
	district := "ولیعصر"
	e.repo.AddStore(model.AdminStoreDetail{
		ID:           adminStoreTestID,
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

func TestAdminStore_GetAndStatusToggle(t *testing.T) {
	env := newAdminStoreEnv(t)
	env.seedStore()

	// Unauthenticated -> 401
	req := httptest.NewRequest(http.MethodGet, "/admin/stores/"+adminStoreTestID, nil)
	rec := httptest.NewRecorder()
	env.r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}

	// Customer -> 403
	req = httptest.NewRequest(http.MethodGet, "/admin/stores/"+adminStoreTestID, nil)
	req.Header.Set("Authorization", "Bearer "+env.tokens["customer"])
	rec = httptest.NewRecorder()
	env.r.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}

	// Admin GET -> 200
	req = httptest.NewRequest(http.MethodGet, "/admin/stores/"+adminStoreTestID, nil)
	req.Header.Set("Authorization", "Bearer "+env.tokens["admin"])
	rec = httptest.NewRecorder()
	env.r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var got dto.AdminStoreDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !got.IsActive || got.Name != "نان صبحگاهی" || len(got.WorkingHours) != 2 {
		t.Fatalf("unexpected response: %+v", got)
	}

	// Admin PATCH /status -> deactivate (200)
	req = httptest.NewRequest(http.MethodPatch, "/admin/stores/"+adminStoreTestID+"/status", bytes.NewBufferString(`{"is_active":false}`))
	req.Header.Set("Authorization", "Bearer "+env.tokens["admin"])
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	env.r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.IsActive {
		t.Fatalf("expected is_active=false")
	}

	// Missing is_active -> 400
	req = httptest.NewRequest(http.MethodPatch, "/admin/stores/"+adminStoreTestID+"/status", bytes.NewBufferString(`{}`))
	req.Header.Set("Authorization", "Bearer "+env.tokens["admin"])
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	env.r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
