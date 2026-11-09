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

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/middleware"
)

type publicStoreServiceStub struct {
	response *dto.PublicStoreResponse
	err      error
	calledID string
}

func (s *publicStoreServiceStub) GetPublicStore(_ context.Context, publicID string) (*dto.PublicStoreResponse, error) {
	s.calledID = publicID
	return s.response, s.err
}

var _ serviceinterface.PublicStoreService = (*publicStoreServiceStub)(nil)

func newPublicStoreTestRouter(h interface{ GetPublicStore(*gin.Context) }) *gin.Engine {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	errorHandler := middleware.NewErrorHandler(log)

	r := gin.New()
	r.Use(errorHandler.Handle())
	return r
}

func TestPublicStoreHandlerEligibleStore(t *testing.T) {
	gin.SetMode(gin.TestMode)

	expected := &dto.PublicStoreResponse{
		PublicID:           "store-123",
		Name:               "نان صبحگاهی",
		CategoryName:       "نانوایی",
		District:           "ولیعصر",
		HeaderSubtitle:     "نانوایی · ولیعصر",
		IsOpen:             true,
		StatusBadge:        "باز است",
		Biography:          "نانوایی محلی با نان‌های تازه و شیرینی‌های پخت روز.",
		PickupAddress:      "تهران، خیابان ولیعصر، پلاک ۱۲",
		PickupInstructions: "تحویل از پیشخوان نانوایی، ورودی خیابان ولیعصر.",
		PickupSummary:      "۳ موجود · تحویل امروز، ۱۸:۰۰-۱۹:۰۰",
		Location: dto.StoreLocationResponse{
			Latitude:  35.6892,
			Longitude: 51.3890,
		},
		WorkingHoursSchedule: []dto.PublicWorkingScheduleItem{
			{Days: "شنبه", Hours: "۰۸:۰۰-۱۹:۰۰", IsClosed: false},
			{Days: "یکشنبه تا چهارشنبه", Hours: "۰۸:۰۰-۱۹:۰۰", IsClosed: false},
			{Days: "پنج‌شنبه و جمعه", Hours: "تعطیل", IsClosed: true},
		},
		Offers: []dto.PublicOfferResponse{
			{
				PublicID:          "off-1",
				Title:             "نان صبحگاهی",
				CategoryName:      "نانوایی",
				DistanceText:      "۰٫۶ کیلومتر",
				OriginalPrice:     115000,
				Price:             40000,
				Currency:          "تومان",
				QuantityAvailable: 3,
				AvailabilityBadge: "۳ بسته موجود",
				PickupWindow:      "امروز، ۱۸:۰۰-۱۹:۰۰",
			},
		},
	}

	svc := &publicStoreServiceStub{response: expected}
	h := NewPublicStoreHandler(svc)

	r := newPublicStoreTestRouter(h)
	r.GET("/stores/:id", h.GetPublicStore)

	req := httptest.NewRequest(http.MethodGet, "/stores/store-123", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	if svc.calledID != "store-123" {
		t.Fatalf("expected store-123, got %s", svc.calledID)
	}

	body := rec.Body.String()

	// Verify required public fields are present
	for _, expectedStr := range []string{
		`"name":"نان صبحگاهی"`,
		`"pickup_address":"تهران، خیابان ولیعصر، پلاک ۱۲"`,
		`"pickup_instructions":"تحویل از پیشخوان نانوایی، ورودی خیابان ولیعصر."`,
		`"header_subtitle":"نانوایی · ولیعصر"`,
		`"status_badge":"باز است"`,
		`"is_open":true`,
		`"offers"`,
		`"working_hours_schedule"`,
		`"original_price":115000`,
		`"price":40000`,
		`"quantity_available":3`,
	} {
		if !strings.Contains(body, expectedStr) {
			t.Fatalf("expected %s in body: %s", expectedStr, body)
		}
	}

	// Verify private fields are strictly absent
	for _, privateField := range []string{
		"vendor_id",
		"trust_score",
		"rating_avg",
		"vendor_status",
		"approval_status",
	} {
		if strings.Contains(body, privateField) {
			t.Fatalf("private field leaked in public response: %s", privateField)
		}
	}
}

func TestPublicStoreHandlerNotFoundOrUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &publicStoreServiceStub{err: exception.NewNotFound("store not found")}
	h := NewPublicStoreHandler(svc)

	r := newPublicStoreTestRouter(h)
	r.GET("/stores/:id", h.GetPublicStore)

	req := httptest.NewRequest(http.MethodGet, "/stores/missing", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "NOT_FOUND") {
		t.Fatalf("expected NOT_FOUND code in error response: %s", rec.Body.String())
	}
}

func TestPublicStoreHandlerStoreWithoutOffers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	expected := &dto.PublicStoreResponse{
		PublicID:           "store-empty",
		Name:               "نان صبحگاهی",
		PickupAddress:      "تهران، خیابان ولیعصر",
		PickupInstructions: "تحویل از پیشخوان",
		Offers:             []dto.PublicOfferResponse{},
	}

	svc := &publicStoreServiceStub{response: expected}
	h := NewPublicStoreHandler(svc)

	r := newPublicStoreTestRouter(h)
	r.GET("/stores/:id", h.GetPublicStore)

	req := httptest.NewRequest(http.MethodGet, "/stores/store-empty", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `"offers":[]`) {
		t.Fatalf("expected empty offers array: %s", body)
	}
}
