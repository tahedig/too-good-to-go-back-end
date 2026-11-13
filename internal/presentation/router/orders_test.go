package router_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	"github.com/tahedig/too-good-to-go/backend/internal/application/service"
	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/bootstrap"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/infrastructure/repository/memory"
	"github.com/tahedig/too-good-to-go/backend/internal/infrastructure/token"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/handler"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/middleware"
	"github.com/tahedig/too-good-to-go/backend/internal/presentation/router"
	"github.com/tahedig/too-good-to-go/backend/pkg/logger"
	"github.com/tahedig/too-good-to-go/backend/pkg/security"
)

type ordersEnv struct {
	r        *gin.Engine
	sessions serviceinterface.SessionService
	users    *memory.UserRepository
	orders   *memory.OrderRepository
}

func newOrdersEnv(t *testing.T) *ordersEnv {
	t.Helper()
	log := logger.New("debug", &bytes.Buffer{})
	users, orders := memory.NewUserRepository(), memory.NewOrderRepository()
	sessions := service.NewSessionService(users, token.NewJWTManager(rbacSecret, rbacIssuer, time.Hour),
		map[string]enum.Role{rbacVendorPhone: enum.RoleVendor}, log)
	r, err := router.New(&bootstrap.Config{}, log, middleware.NewErrorHandler(log), middleware.NewAuthMiddleware(sessions), nil,
		router.Handlers{
			Health: handler.NewHealthHandler(service.NewHealthService("test", time.Second, log)),
			Auth:   handler.NewAuthHandler(nil, nil),
			Order:  handler.NewOrderHandler(service.NewOrderService(orders)),
		})
	if err != nil {
		t.Fatal(err)
	}
	return &ordersEnv{r: r, sessions: sessions, users: users, orders: orders}
}

func (e *ordersEnv) login(t *testing.T, phone string) *serviceinterface.AuthResult {
	t.Helper()
	res, err := e.sessions.Login(context.Background(), phone)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func (e *ordersEnv) list(t *testing.T, tok string, query url.Values) (*httptest.ResponseRecorder, dto.OrderListResponse) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/orders?"+query.Encode(), nil)
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	var body dto.OrderListResponse
	if w.Code == http.StatusOK {
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v: %s", err, w.Body.String())
		}
	}
	return w, body
}

func (e *ordersEnv) addOrder(t *testing.T, userID string, status enum.OrderStatus, createdAt time.Time) string {
	t.Helper()
	id, err := security.NewID()
	if err != nil {
		t.Fatal(err)
	}
	pickup := createdAt.Add(4 * time.Hour)
	end := pickup.Add(time.Hour)
	e.orders.Add(userID, model.Order{
		ID: id, Status: status, CreatedAt: createdAt, ReservedUntil: createdAt.Add(10 * time.Minute),
		Store:      model.OrderStore{ID: "550e8400-e29b-41d4-a716-446655440000", Name: "نان صبحگاهی", City: "تهران", AddressLine: "خیابان ولیعصر، پلاک ۱۲"},
		Items:      []model.OrderItem{{Title: "Bread surprise bag", OfferType: "surprise_bag", Quantity: 1, UnitPrice: 40000}},
		ItemsTotal: 40000, TotalAmount: 40000, PickupStart: &pickup, PickupEnd: &end,
	})
	return id
}

func TestOrdersRequireAuthentication(t *testing.T) {
	e := newOrdersEnv(t)
	for _, tok := range []string{"", "garbage"} {
		if w, _ := e.list(t, tok, nil); w.Code != http.StatusUnauthorized {
			t.Errorf("token %q: got %d, want 401", tok, w.Code)
		}
	}
}

func TestOrdersResponseShape(t *testing.T) {
	e := newOrdersEnv(t)
	alice := e.login(t, rbacCustomerPhone)
	at := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	paid := e.addOrder(t, alice.User.ID, enum.OrderStatusPaid, at)
	pending := e.addOrder(t, alice.User.ID, enum.OrderStatusPendingPayment, at.Add(time.Minute))

	w, body := e.list(t, alice.AccessToken, nil)
	if w.Code != http.StatusOK || len(body.Orders) != 2 || body.NextCursor != nil {
		t.Fatalf("got %d %s", w.Code, w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Error("order history must not be cached")
	}
	p, o := body.Orders[0], body.Orders[1]
	if p.ID != pending || p.Group != "upcoming" || p.ReservedUntil == nil || p.PaymentStatus != nil {
		t.Fatalf("pending order: %+v", p)
	}
	if o.ID != paid || o.Status != "paid" || o.ReservedUntil != nil || o.Store.PickupAddress != "تهران، خیابان ولیعصر، پلاک ۱۲" ||
		len(o.Items) != 1 || o.Items[0].OfferType != "surprise_bag" || o.TotalAmount != 40000 || o.PickupStart == nil {
		t.Fatalf("paid order: %+v", o)
	}
}

func TestOrdersAreScopedToTheCaller(t *testing.T) {
	e := newOrdersEnv(t)
	alice, vendor := e.login(t, rbacCustomerPhone), e.login(t, rbacVendorPhone)
	at := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	aliceOrder := e.addOrder(t, alice.User.ID, enum.OrderStatusCompleted, at)
	vendorOrder := e.addOrder(t, vendor.User.ID, enum.OrderStatusPaid, at)

	if _, body := e.list(t, alice.AccessToken, nil); len(body.Orders) != 1 || body.Orders[0].ID != aliceOrder {
		t.Fatalf("alice: %+v", body.Orders)
	}
	// Vendors may buy too, and see only their own purchases.
	if _, body := e.list(t, vendor.AccessToken, nil); len(body.Orders) != 1 || body.Orders[0].ID != vendorOrder {
		t.Fatalf("vendor: %+v", body.Orders)
	}

	// An empty history is a 200 with an empty array, not null.
	bob := e.login(t, "+989125555555")
	w, _ := e.list(t, bob.AccessToken, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"orders":[]`) || !strings.Contains(w.Body.String(), `"next_cursor":null`) {
		t.Fatalf("empty history: %d %s", w.Code, w.Body.String())
	}

	e.users.SetAccess(alice.User.ID, enum.RoleCustomer, model.UserStatusSuspended)
	if w, _ := e.list(t, alice.AccessToken, nil); w.Code != http.StatusForbidden {
		t.Fatalf("suspended user: got %d, want 403", w.Code)
	}
}

func TestOrdersPaginateAndFilterOverHTTP(t *testing.T) {
	e := newOrdersEnv(t)
	alice := e.login(t, rbacCustomerPhone)
	at := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	var want []string
	for i, s := range []enum.OrderStatus{enum.OrderStatusCompleted, enum.OrderStatusNoShow, enum.OrderStatusPaid, enum.OrderStatusReady, enum.OrderStatusCancelled} {
		want = append([]string{e.addOrder(t, alice.User.ID, s, at.Add(time.Duration(i)*time.Hour))}, want...)
	}

	var got []string
	q := url.Values{"limit": {"2"}}
	for {
		w, body := e.list(t, alice.AccessToken, q)
		if w.Code != http.StatusOK {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
		for _, o := range body.Orders {
			got = append(got, o.ID)
		}
		if body.NextCursor == nil {
			break
		}
		q.Set("cursor", *body.NextCursor)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("pages: got %v, want %v", got, want)
	}

	_, body := e.list(t, alice.AccessToken, url.Values{"group": {"upcoming"}})
	if len(body.Orders) != 2 || body.Orders[0].Status != "ready" || body.Orders[1].Status != "paid" {
		t.Fatalf("upcoming: %+v", body.Orders)
	}
	_, body = e.list(t, alice.AccessToken, url.Values{"status": {"no_show,completed"}})
	if len(body.Orders) != 2 || body.Orders[0].Group != "past" {
		t.Fatalf("status filter: %+v", body.Orders)
	}

	w, _ := e.list(t, alice.AccessToken, url.Values{"limit": {"500"}, "group": {"soon"}})
	if w.Code != http.StatusBadRequest || decodeError(t, w).Code != exception.CodeBadRequest ||
		!strings.Contains(w.Body.String(), `"limit"`) || !strings.Contains(w.Body.String(), `"group"`) {
		t.Fatalf("bad query: %d %s", w.Code, w.Body.String())
	}
}
