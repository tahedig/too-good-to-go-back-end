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

const (
	vendorStoreOne    = "10000000-0000-4000-8000-000000000001"
	vendorStoreTwo    = "10000000-0000-4000-8000-000000000002"
	otherVendorStore  = "10000000-0000-4000-8000-000000000099"
	secondVendorPhone = "+989121111199"
)

type vendorDashEnv struct {
	r      *gin.Engine
	repo   *memory.VendorDashboardRepository
	tokens map[string]string
	ids    map[string]string
}

func newVendorDashEnv(t *testing.T) *vendorDashEnv {
	t.Helper()
	log := logger.New("debug", &bytes.Buffer{})
	sessions := service.NewSessionService(
		memory.NewUserRepository(),
		token.NewJWTManager(rbacSecret, rbacIssuer, time.Hour),
		map[string]enum.Role{
			rbacVendorPhone:   enum.RoleVendor,
			secondVendorPhone: enum.RoleVendor,
			rbacAdminPhone:    enum.RoleAdmin,
		},
		log,
	)
	repo := memory.NewVendorDashboardRepository()
	r, err := router.New(
		&bootstrap.Config{},
		log,
		middleware.NewErrorHandler(log),
		middleware.NewAuthMiddleware(sessions),
		nil,
		router.Handlers{
			Health:          handler.NewHealthHandler(service.NewHealthService("test", time.Second, log)),
			Auth:            handler.NewAuthHandler(nil, nil),
			VendorDashboard: handler.NewVendorDashboardHandler(service.NewVendorDashboardService(repo)),
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	env := &vendorDashEnv{
		r:      r,
		repo:   repo,
		tokens: make(map[string]string),
		ids:    make(map[string]string),
	}
	for role, phone := range map[string]string{
		"customer": rbacCustomerPhone,
		"vendor":   rbacVendorPhone,
		"vendor2":  secondVendorPhone,
		"admin":    rbacAdminPhone,
	} {
		res, err := sessions.Login(context.Background(), phone)
		if err != nil {
			t.Fatal(err)
		}
		env.tokens[role] = res.AccessToken
		env.ids[role] = res.User.ID
	}

	now := time.Now().UTC()
	pickupStart := now.Add(2 * time.Hour)
	pickupEnd := now.Add(3 * time.Hour)
	img := "https://example.com/bag.jpg"

	repo.SetCommissionRate(env.ids["vendor"], 10)
	repo.AddStore(env.ids["vendor"], model.VendorDashboardStore{
		ID: vendorStoreOne, Name: "نان صبحگاهی", City: "تهران", AddressLine: "خیابان ولیعصر، پلاک ۱۲",
		Status: enum.StoreStatusApproved, AcceptsOrders: true, CreatedAt: now.Add(-48 * time.Hour),
	})
	repo.AddStore(env.ids["vendor"], model.VendorDashboardStore{
		ID: vendorStoreTwo, Name: "نان صبحگاهی · شعبه شریعتی", City: "تهران", AddressLine: "خیابان شریعتی، پلاک ۲۴",
		Status: enum.StoreStatusPending, AcceptsOrders: true, CreatedAt: now.Add(-24 * time.Hour),
	})
	repo.AddStore(env.ids["vendor2"], model.VendorDashboardStore{
		ID: otherVendorStore, Name: "فروشگاه دیگر", City: "تهران", AddressLine: "میدان ونک",
		Status: enum.StoreStatusApproved, AcceptsOrders: true, CreatedAt: now.Add(-48 * time.Hour),
	})

	repo.AddOffer(memory.VendorDashboardOffer{
		ID: "off-1", StoreID: vendorStoreOne, Title: "بسته غافلگیری نانوایی", ImageURL: &img,
		Status: "active", QuantityTotal: 6, QuantityAvailable: 3, PickupStart: pickupStart, PickupEnd: pickupEnd,
	})
	repo.AddOffer(memory.VendorDashboardOffer{
		ID: "off-other", StoreID: otherVendorStore, Title: "آفر رقیب",
		Status: "active", QuantityTotal: 10, QuantityAvailable: 9, PickupStart: pickupStart, PickupEnd: pickupEnd,
	})

	repo.AddReservation(memory.VendorDashboardRecord{
		ID: "20000000-0000-4000-8000-000000000001", StoreID: vendorStoreOne, Status: enum.OrderStatusPaid,
		PaymentStatus: "succeeded", PickupCode: "4821", CustomerName: "لیلا علیزاده", OfferID: "off-1",
		Items:      []model.OrderItem{{Title: "بسته غافلگیری نانوایی", OfferType: "surprise_bag", Quantity: 1, UnitPrice: 40000}},
		ItemsTotal: 40000, TotalAmount: 40000, VendorPayout: 36000,
		PickupStart: &pickupStart, PickupEnd: &pickupEnd, CreatedAt: now.Add(-15 * time.Minute),
	})
	completedAt := now.Add(-5 * time.Minute)
	repo.AddReservation(memory.VendorDashboardRecord{
		ID: "20000000-0000-4000-8000-000000000002", StoreID: vendorStoreOne, Status: enum.OrderStatusCompleted,
		PaymentStatus: "succeeded", PickupCode: "1122", CustomerName: "سارا رضایی", OfferID: "off-1",
		Items:      []model.OrderItem{{Title: "بسته غافلگیری نانوایی", OfferType: "surprise_bag", Quantity: 2, UnitPrice: 40000}},
		ItemsTotal: 80000, TotalAmount: 80000, VendorPayout: 72000,
		PickupStart: &pickupStart, PickupEnd: &pickupEnd, CompletedAt: &completedAt, CreatedAt: now.Add(-2 * time.Minute),
	})
	repo.AddReservation(memory.VendorDashboardRecord{
		ID: "20000000-0000-4000-8000-000000000099", StoreID: otherVendorStore, Status: enum.OrderStatusPaid,
		PaymentStatus: "succeeded", PickupCode: "9999", CustomerName: "مشتری دیگر", OfferID: "off-other",
		Items:      []model.OrderItem{{Title: "آفر رقیب", OfferType: "surprise_bag", Quantity: 5, UnitPrice: 50000}},
		ItemsTotal: 250000, TotalAmount: 250000, VendorPayout: 225000,
		PickupStart: &pickupStart, PickupEnd: &pickupEnd, CreatedAt: now.Add(-1 * time.Minute),
	})
	return env
}

func (e *vendorDashEnv) call(path, role string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if role != "" {
		req.Header.Set("Authorization", "Bearer "+e.tokens[role])
	}
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	return w
}

func TestVendorDashboardRBAC(t *testing.T) {
	e := newVendorDashEnv(t)
	for role, want := range map[string]int{
		"":         http.StatusUnauthorized,
		"customer": http.StatusForbidden,
		"admin":    http.StatusForbidden,
	} {
		if w := e.call("/vendor/dashboard", role); w.Code != want {
			t.Errorf("GET /vendor/dashboard as %q: got %d, want %d", role, w.Code, want)
		}
	}
}

func TestVendorDashboardOwnershipAndMetrics(t *testing.T) {
	e := newVendorDashEnv(t)
	w := e.call("/vendor/dashboard", "vendor")
	if w.Code != http.StatusOK {
		t.Fatalf("vendor dashboard: %d %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}

	var resp dto.VendorDashboardResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.ActiveStore == nil || resp.ActiveStore.ID != vendorStoreOne || len(resp.Stores) != 2 {
		t.Fatalf("stores: active=%+v list=%+v", resp.ActiveStore, resp.Stores)
	}
	if len(resp.PendingStoreRequests) != 1 || resp.PendingStoreRequests[0].ID != vendorStoreTwo {
		t.Fatalf("pending store requests: %+v", resp.PendingStoreRequests)
	}
	if resp.KPIs.ActiveOffers != 1 || resp.KPIs.AvailablePackages != 3 ||
		resp.KPIs.ReservedPackages != 1 || resp.KPIs.ReceivedRevenue != 72000 {
		t.Fatalf("kpis: %+v", resp.KPIs)
	}
	if resp.ReservationCounts.Paid != 1 || resp.ReservationCounts.Completed != 1 ||
		resp.ReservationCounts.Active != 1 || resp.ReservationCounts.Total != 2 {
		t.Fatalf("counts: %+v", resp.ReservationCounts)
	}
	if resp.Today.ReservationsCount != 2 || resp.Today.PackagesReserved != 3 ||
		resp.Today.CompletedCount != 1 || resp.Today.Revenue != 108000 {
		t.Fatalf("today: %+v", resp.Today)
	}
	if resp.Financial.ReceivedPayout != 72000 || resp.Financial.PendingPayout != 36000 ||
		resp.Financial.NetPeriodRevenue != 108000 || resp.Financial.Plan.CommissionRate != 10 {
		t.Fatalf("financial: %+v", resp.Financial)
	}
	if resp.NearestReservation == nil || resp.NearestReservation.ID != "20000000-0000-4000-8000-000000000001" ||
		resp.NearestReservation.OfferQuantityAvailable != 3 || resp.NearestReservation.OfferQuantityReserved != 3 ||
		resp.NearestReservation.OfferQuantityTotal != 6 {
		t.Fatalf("nearest: %+v", resp.NearestReservation)
	}
	if len(resp.RecentReservations) != 2 || resp.RecentReservations[0].ID != "20000000-0000-4000-8000-000000000002" {
		t.Fatalf("recent reservations: %+v", resp.RecentReservations)
	}

	if w := e.call("/vendor/dashboard?store_id="+otherVendorStore, "vendor"); w.Code != http.StatusNotFound {
		t.Fatalf("other vendor store_id: got %d, want 404 (%s)", w.Code, w.Body.String())
	}

	w = e.call("/vendor/dashboard?store_id="+vendorStoreOne+"&recent_limit=1", "vendor")
	var scoped dto.VendorDashboardResponse
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &scoped) != nil || len(scoped.RecentReservations) != 1 {
		t.Fatalf("scoped dashboard: %d %s", w.Code, w.Body.String())
	}

	if w := e.call("/vendor/dashboard?store_id=bad&recent_limit=99", "vendor"); w.Code != http.StatusBadRequest {
		t.Fatalf("bad query: got %d, want 400", w.Code)
	}
}
