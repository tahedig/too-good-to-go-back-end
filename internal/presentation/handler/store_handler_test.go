package handler

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/bootstrap"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/middleware"
)

type storeServiceStub struct {
	called   bool
	userID   int64
	publicID string
	updated  *model.Store
}

func (s *storeServiceStub) ListStores(_ context.Context, userID int64) ([]*model.Store, error) {
	s.called = true
	s.userID = userID
	return []*model.Store{}, nil
}

func (s *storeServiceStub) GetStore(_ context.Context, userID int64, publicID string) (*model.Store, error) {
	s.called = true
	s.userID = userID
	s.publicID = publicID

	return &model.Store{
		PublicID: publicID,
		Name:     "Test Store",
	}, nil
}

func (s *storeServiceStub) UpdateStore(
	_ context.Context,
	userID int64,
	publicID string,
	updated *model.Store,
) (*model.Store, error) {
	s.called = true
	s.userID = userID
	s.publicID = publicID
	s.updated = updated

	updated.PublicID = publicID
	return updated, nil
}

var _ serviceinterface.StoreService = (*storeServiceStub)(nil)

func TestStoreHandlerRejectsUnauthenticatedListRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &storeServiceStub{}
	h := NewStoreHandler(svc)

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	errorHandler := middleware.NewErrorHandler(log)

	r := gin.New()
	r.Use(errorHandler.Handle())
	r.GET("/stores", h.ListStores)

	req := httptest.NewRequest(http.MethodGet, "/stores", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "UNAUTHORIZED") {
		t.Fatalf("expected UNAUTHORIZED error code, got: %s", rec.Body.String())
	}

	if svc.called {
		t.Fatal("store service should not be called for an unauthenticated request")
	}
}

func TestStoreHandlerListsStoresForAuthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &storeServiceStub{}
	h := NewStoreHandler(svc)

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	errorHandler := middleware.NewErrorHandler(log)

	r := gin.New()
	r.Use(errorHandler.Handle())
	r.GET("/stores", func(c *gin.Context) {
		c.Set(bootstrap.CtxKeyUserID, int64(42))
		h.ListStores(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/stores", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	if !svc.called {
		t.Fatal("expected store service to be called")
	}

	if svc.userID != 42 {
		t.Fatalf("expected user ID 42, got %d", svc.userID)
	}
}

func TestStoreHandlerGetsStoreForAuthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &storeServiceStub{}
	h := NewStoreHandler(svc)

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	errorHandler := middleware.NewErrorHandler(log)

	r := gin.New()
	r.Use(errorHandler.Handle())
	r.GET("/stores/:storeID", func(c *gin.Context) {
		c.Set(bootstrap.CtxKeyUserID, int64(42))
		h.GetStore(c)
	})

	publicID := "550e8400-e29b-41d4-a716-446655440000"

	req := httptest.NewRequest(http.MethodGet, "/stores/"+publicID, nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	if !svc.called {
		t.Fatal("expected store service to be called")
	}

	if svc.userID != 42 {
		t.Fatalf("expected user ID 42, got %d", svc.userID)
	}

	if svc.publicID != publicID {
		t.Fatalf("expected public ID %s, got %s", publicID, svc.publicID)
	}

	if !strings.Contains(rec.Body.String(), `"name":"Test Store"`) {
		t.Fatalf("expected store name in response, got: %s", rec.Body.String())
	}
}

func TestStoreHandlerUpdatesOnlyEditableFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &storeServiceStub{}
	h := NewStoreHandler(svc)

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	errorHandler := middleware.NewErrorHandler(log)

	r := gin.New()
	r.Use(errorHandler.Handle())
	r.PUT("/stores/:storeID", func(c *gin.Context) {
		c.Set(bootstrap.CtxKeyUserID, int64(42))
		h.UpdateStore(c)
	})

	publicID := "550e8400-e29b-41d4-a716-446655440000"

	body := `{
		"name": "Updated Store",
		"biography": "Fresh groceries",
		"pickup_address": "Tehran, Example Street",
		"pickup_instructions": "Ring the bell",
		"vendor_id": 999,
		"rating_avg": 5.0,
		"trust_score": 100,
		"approval_status": "approved"
	}`

	req := httptest.NewRequest(http.MethodPut, "/stores/"+publicID, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	if !svc.called {
		t.Fatal("expected store service to be called")
	}

	if svc.userID != 42 {
		t.Fatalf("expected user ID 42, got %d", svc.userID)
	}

	if svc.publicID != publicID {
		t.Fatalf("expected public ID %s, got %s", publicID, svc.publicID)
	}

	if svc.updated == nil {
		t.Fatal("expected updated store data")
	}

	if svc.updated.Name != "Updated Store" ||
		svc.updated.Biography != "Fresh groceries" ||
		svc.updated.PickupAddress != "Tehran, Example Street" ||
		svc.updated.PickupInstructions != "Ring the bell" {
		t.Fatalf("unexpected editable fields: %+v", svc.updated)
	}

	if svc.updated.VendorID != 0 ||
		svc.updated.RatingAvg != 0 ||
		svc.updated.TrustScore != 0 ||
		svc.updated.ApprovalStatus != "" {
		t.Fatalf("non-editable fields should not be populated from request: %+v", svc.updated)
	}
}
