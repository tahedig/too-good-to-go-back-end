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
	phone := "09124550120"
	catID := int16(1)

	return []*model.Store{
		{
			PublicID:            "store-1",
			Name:                "نان صبحگاهی",
			CategoryID:          &catID,
			CategoryName:        "نانوایی",
			OwnerName:           "آیدا کریمی",
			Phone:               &phone,
			Biography:           "نانوایی محلی با نان‌های تازه",
			PickupInstructions:  "تحویل از پیشخوان نانوایی",
			AddressLine:         "تهران، خیابان ولیعصر، پلاک ۱۲",
			Location:            model.StoreLocation{Latitude: 35.6892, Longitude: 51.3890},
			VendorStatus:        "approved",
			IsActive:            true,
			AcceptsOrders:       true,
			ActiveProductsCount: 1,
			RatingAvg:           4.5,
			TrustScore:          90,
			WorkingHours: []model.WorkingHour{
				{DayOfWeek: 0, DayNameFa: "شنبه", OpenTime: "08:00", CloseTime: "19:00", IsClosed: false},
				{DayOfWeek: 4, DayNameFa: "چهارشنبه", OpenTime: "08:00", CloseTime: "19:00", IsClosed: false},
				{DayOfWeek: 5, DayNameFa: "پنج‌شنبه", IsClosed: true},
				{DayOfWeek: 6, DayNameFa: "جمعه", IsClosed: true},
			},
			Images: []model.StoreImage{
				{ID: "img-1", URL: "https://example.com/cover.jpg", IsPrimary: true, SortOrder: 1},
			},
		},
	}, nil
}

func (s *storeServiceStub) GetStore(_ context.Context, userID int64, publicID string) (*model.Store, error) {
	s.called = true
	s.userID = userID
	s.publicID = publicID
	phone := "09124550120"
	catID := int16(1)

	return &model.Store{
		PublicID:            publicID,
		Name:                "نان صبحگاهی",
		CategoryID:          &catID,
		CategoryName:        "نانوایی",
		OwnerName:           "آیدا کریمی",
		Phone:               &phone,
		Biography:           "نانوایی محلی با نان‌های تازه",
		PickupInstructions:  "تحویل از پیشخوان نانوایی، ورودی خیابان ولیعصر.",
		AddressLine:         "تهران، خیابان ولیعصر، پلاک ۱۲",
		Location:            model.StoreLocation{Latitude: 35.6892, Longitude: 51.3890},
		VendorStatus:        "approved",
		IsActive:            true,
		AcceptsOrders:       true,
		ActiveProductsCount: 1,
		RatingAvg:           4.5,
		TrustScore:          90,
		WorkingHours: []model.WorkingHour{
			{DayOfWeek: 0, DayNameFa: "شنبه", OpenTime: "08:00", CloseTime: "19:00", IsClosed: false},
			{DayOfWeek: 4, DayNameFa: "چهارشنبه", OpenTime: "08:00", CloseTime: "19:00", IsClosed: false},
			{DayOfWeek: 5, DayNameFa: "پنج‌شنبه", IsClosed: true},
			{DayOfWeek: 6, DayNameFa: "جمعه", IsClosed: true},
		},
		Images: []model.StoreImage{
			{ID: "img-1", URL: "https://example.com/cover.jpg", IsPrimary: true, SortOrder: 1},
		},
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

func newStoreHandlerTestRouter(h interface {
	ListStores(*gin.Context)
	GetStore(*gin.Context)
	UpdateStore(*gin.Context)
}) *gin.Engine {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	errorHandler := middleware.NewErrorHandler(log)

	r := gin.New()
	r.Use(errorHandler.Handle())
	return r
}

func TestStoreHandlerRejectsUnauthenticatedListRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &storeServiceStub{}
	h := NewStoreHandler(svc)

	r := newStoreHandlerTestRouter(h)
	r.GET("/vendor/stores", h.ListStores)

	req := httptest.NewRequest(http.MethodGet, "/vendor/stores", nil)
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

func TestStoreHandlerRejectsUnauthenticatedGetRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &storeServiceStub{}
	h := NewStoreHandler(svc)

	r := newStoreHandlerTestRouter(h)
	r.GET("/vendor/stores/:id", h.GetStore)

	req := httptest.NewRequest(http.MethodGet, "/vendor/stores/store-123", nil)
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

func TestStoreHandlerRejectsUnauthenticatedUpdateRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &storeServiceStub{}
	h := NewStoreHandler(svc)

	r := newStoreHandlerTestRouter(h)
	r.PATCH("/vendor/stores/:id", h.UpdateStore)

	body := `{"name": "New Name", "pickup_address": "New Address"}`
	req := httptest.NewRequest(http.MethodPatch, "/vendor/stores/store-123", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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

	r := newStoreHandlerTestRouter(h)
	r.GET("/vendor/stores", func(c *gin.Context) {
		c.Set(bootstrap.CtxKeyUserID, int64(42))
		h.ListStores(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/vendor/stores", nil)
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

	resp := rec.Body.String()
	for _, expected := range []string{
		`"name":"نان صبحگاهی"`,
		`"category_name":"نانوایی"`,
		`"owner_name":"آیدا کریمی"`,
		`"pickup_address":"تهران، خیابان ولیعصر، پلاک ۱۲"`,
		`"working_hours_summary":"شنبه تا چهارشنبه · ۰۸:۰۰ تا ۱۹:۰۰"`,
		`"closed_days_summary":"پنج‌شنبه و جمعه تعطیل"`,
		`"active_products_count":1`,
	} {
		if !strings.Contains(resp, expected) {
			t.Fatalf("expected %s in response, got: %s", expected, resp)
		}
	}
}

func TestStoreHandlerGetsStoreForAuthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &storeServiceStub{}
	h := NewStoreHandler(svc)

	r := newStoreHandlerTestRouter(h)
	r.GET("/vendor/stores/:id", func(c *gin.Context) {
		c.Set(bootstrap.CtxKeyUserID, int64(42))
		h.GetStore(c)
	})

	publicID := "550e8400-e29b-41d4-a716-446655440000"

	req := httptest.NewRequest(http.MethodGet, "/vendor/stores/"+publicID, nil)
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

	response := rec.Body.String()

	for _, expected := range []string{
		`"name":"نان صبحگاهی"`,
		`"biography":"نانوایی محلی با نان‌های تازه"`,
		`"pickup_address":"تهران، خیابان ولیعصر، پلاک ۱۲"`,
		`"pickup_instructions":"تحویل از پیشخوان نانوایی، ورودی خیابان ولیعصر."`,
		`"approval_status":"approved"`,
		`"availability_status":"available"`,
		`"rating_avg":4.5`,
		`"trust_score":90`,
		`"latitude":35.6892`,
		`"longitude":51.389`,
		`"owner_name":"آیدا کریمی"`,
		`"category_name":"نانوایی"`,
		`"working_hours_summary":"شنبه تا چهارشنبه · ۰۸:۰۰ تا ۱۹:۰۰"`,
		`"closed_days_summary":"پنج‌شنبه و جمعه تعطیل"`,
	} {
		if !strings.Contains(response, expected) {
			t.Fatalf("expected %s in response, got: %s", expected, response)
		}
	}
}

func TestStoreHandlerUpdatesOnlyEditableFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &storeServiceStub{}
	h := NewStoreHandler(svc)

	r := newStoreHandlerTestRouter(h)
	r.PUT("/vendor/stores/:id", func(c *gin.Context) {
		c.Set(bootstrap.CtxKeyUserID, int64(42))
		h.UpdateStore(c)
	})

	publicID := "550e8400-e29b-41d4-a716-446655440000"

	body := `{
		"name": "Updated Store",
		"pickup_address": "Tehran, Example Street",
		"biography": "Fresh groceries",
		"pickup_instructions": "Ring the bell",
		"phone": "09124550120",
		"category_id": 1,
		"location": {"latitude": 35.7, "longitude": 51.4},
		"working_hours": [
			{"day_of_week": 0, "open_time": "08:00", "close_time": "19:00", "is_closed": false},
			{"day_of_week": 5, "is_closed": true}
		],
		"images": [
			{"id": "img-1", "url": "https://example.com/1.jpg", "is_primary": true, "sort_order": 1}
		],
		"vendor_id": 999,
		"rating_avg": 5.0,
		"trust_score": 100,
		"approval_status": "approved"
	}`

	req := httptest.NewRequest(http.MethodPut, "/vendor/stores/"+publicID, strings.NewReader(body))
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
		svc.updated.AddressLine != "Tehran, Example Street" ||
		svc.updated.Biography != "Fresh groceries" ||
		svc.updated.PickupInstructions != "Ring the bell" ||
		*svc.updated.Phone != "09124550120" ||
		*svc.updated.CategoryID != 1 ||
		svc.updated.Location.Latitude != 35.7 ||
		len(svc.updated.WorkingHours) != 2 ||
		len(svc.updated.Images) != 1 {
		t.Fatalf("unexpected editable fields: %+v", svc.updated)
	}

	// Protected fields must not be populated from the request
	if svc.updated.VendorID != 0 ||
		svc.updated.RatingAvg != 0 ||
		svc.updated.TrustScore != 0 ||
		svc.updated.VendorStatus != "" ||
		svc.updated.IsActive ||
		svc.updated.AcceptsOrders {
		t.Fatalf("non-editable fields should not be populated from request: %+v", svc.updated)
	}
}

func TestStoreHandlerPatchVendorStores(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &storeServiceStub{}
	h := NewStoreHandler(svc)

	r := newStoreHandlerTestRouter(h)
	r.PATCH("/vendor/stores/:id", func(c *gin.Context) {
		c.Set(bootstrap.CtxKeyUserID, int64(42))
		h.UpdateStore(c)
	})

	publicID := "550e8400-e29b-41d4-a716-446655440000"

	body := `{"name": "Patched Store", "pickup_address": "Patched Address"}`

	req := httptest.NewRequest(http.MethodPatch, "/vendor/stores/"+publicID, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	if !svc.called {
		t.Fatal("expected store service to be called")
	}

	if svc.publicID != publicID {
		t.Fatalf("expected public ID %s, got %s", publicID, svc.publicID)
	}

	if svc.updated.Name != "Patched Store" || svc.updated.AddressLine != "Patched Address" {
		t.Fatalf("unexpected updated fields: %+v", svc.updated)
	}
}

func TestStoreHandlerUpdateInvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &storeServiceStub{}
	h := NewStoreHandler(svc)

	r := newStoreHandlerTestRouter(h)
	r.PATCH("/vendor/stores/:id", func(c *gin.Context) {
		c.Set(bootstrap.CtxKeyUserID, int64(42))
		h.UpdateStore(c)
	})

	req := httptest.NewRequest(http.MethodPatch, "/vendor/stores/store-1", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}
