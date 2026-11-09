package handler

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/bootstrap"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/middleware"
)

type storeRequestServiceStub struct {
	called   bool
	userID   int64
	publicID string
	created  *model.StoreRequest
}

func (s *storeRequestServiceStub) ListStoreRequests(_ context.Context, userID int64) ([]*model.StoreRequest, error) {
	s.called = true
	s.userID = userID
	catID := int16(1)

	return []*model.StoreRequest{
		{
			PublicID:     "req-1",
			VendorID:     userID,
			StoreName:    "نان صبحگاهی - شعبه شریعتی",
			CategoryID:   &catID,
			CategoryName: "نانوایی",
			OwnerName:    "آیدا کریمی",
			Address:      "تهران، خیابان شریعتی",
			Status:       "pending_review",
			StatusFa:     "در انتظار بررسی",
			CreatedAt:    time.Now().UTC(),
		},
	}, nil
}

func (s *storeRequestServiceStub) GetStoreRequest(_ context.Context, userID int64, publicID string) (*model.StoreRequest, error) {
	s.called = true
	s.userID = userID
	s.publicID = publicID
	catID := int16(1)

	return &model.StoreRequest{
		PublicID:     publicID,
		VendorID:     userID,
		StoreName:    "نان صبحگاهی - شعبه شریعتی",
		CategoryID:   &catID,
		CategoryName: "نانوایی",
		OwnerName:    "آیدا کریمی",
		Address:      "تهران، خیابان شریعتی",
		Status:       "pending_review",
		StatusFa:     "در انتظار بررسی",
		CreatedAt:    time.Now().UTC(),
	}, nil
}

func (s *storeRequestServiceStub) CreateStoreRequest(
	_ context.Context,
	userID int64,
	req *model.StoreRequest,
) (*model.StoreRequest, error) {
	s.called = true
	s.userID = userID
	s.created = req

	req.PublicID = "req-new"
	req.VendorID = userID
	req.Status = "pending_review"
	req.StatusFa = "در انتظار بررسی"
	req.CreatedAt = time.Now().UTC()
	return req, nil
}

var _ serviceinterface.StoreRequestService = (*storeRequestServiceStub)(nil)

func newStoreRequestHandlerTestRouter(h interface {
	ListStoreRequests(*gin.Context)
	GetStoreRequest(*gin.Context)
	CreateStoreRequest(*gin.Context)
}) *gin.Engine {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	errorHandler := middleware.NewErrorHandler(log)

	r := gin.New()
	r.Use(errorHandler.Handle())
	return r
}

func TestStoreRequestHandlerRejectsUnauthenticated(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &storeRequestServiceStub{}
	h := NewStoreRequestHandler(svc)

	r := newStoreRequestHandlerTestRouter(h)
	r.GET("/vendor/store-requests", h.ListStoreRequests)
	r.GET("/vendor/store-requests/:id", h.GetStoreRequest)
	r.POST("/vendor/store-requests", h.CreateStoreRequest)

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/vendor/store-requests"},
		{http.MethodGet, "/vendor/store-requests/req-1"},
		{http.MethodPost, "/vendor/store-requests"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d", rec.Code)
			}
		})
	}
}

func TestStoreRequestHandlerListRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &storeRequestServiceStub{}
	h := NewStoreRequestHandler(svc)

	r := newStoreRequestHandlerTestRouter(h)
	r.GET("/vendor/store-requests", func(c *gin.Context) {
		c.Set(bootstrap.CtxKeyUserID, int64(42))
		h.ListStoreRequests(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/vendor/store-requests", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if !svc.called || svc.userID != 42 {
		t.Fatal("expected service call with user ID 42")
	}

	body := rec.Body.String()
	for _, expected := range []string{
		`"store_name":"نان صبحگاهی - شعبه شریعتی"`,
		`"category_name":"نانوایی"`,
		`"owner_name":"آیدا کریمی"`,
		`"address":"تهران، خیابان شریعتی"`,
		`"status":"pending_review"`,
		`"status_fa":"در انتظار بررسی"`,
		`"notice":"این شعبه پس از تأیید، امکان انتشار محصول خواهد داشت."`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected %s in response body: %s", expected, body)
		}
	}
}

func TestStoreRequestHandlerGetRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &storeRequestServiceStub{}
	h := NewStoreRequestHandler(svc)

	r := newStoreRequestHandlerTestRouter(h)
	r.GET("/vendor/store-requests/:id", func(c *gin.Context) {
		c.Set(bootstrap.CtxKeyUserID, int64(42))
		h.GetStoreRequest(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/vendor/store-requests/req-123", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if svc.publicID != "req-123" {
		t.Fatalf("expected req-123, got %s", svc.publicID)
	}
}

func TestStoreRequestHandlerCreateRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &storeRequestServiceStub{}
	h := NewStoreRequestHandler(svc)

	r := newStoreRequestHandlerTestRouter(h)
	r.POST("/vendor/store-requests", func(c *gin.Context) {
		c.Set(bootstrap.CtxKeyUserID, int64(42))
		h.CreateStoreRequest(c)
	})

	payload := `{
		"store_name": "نان صبحگاهی - شعبه شریعتی",
		"category_id": 1,
		"owner_name": "آیدا کریمی",
		"address": "تهران، خیابان شریعتی"
	}`

	req := httptest.NewRequest(http.MethodPost, "/vendor/store-requests", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	if svc.created == nil || svc.created.StoreName != "نان صبحگاهی - شعبه شریعتی" {
		t.Fatalf("request not passed to service properly: %+v", svc.created)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `"status":"pending_review"`) || !strings.Contains(body, `"status_fa":"در انتظار بررسی"`) {
		t.Fatalf("expected pending status in response: %s", body)
	}
}
