package router_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	"github.com/tahedig/too-good-to-go/backend/internal/application/service"
	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
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

type reservationsEnv struct {
	r            *gin.Engine
	sessions     serviceinterface.SessionService
	orders       *memory.OrderRepository
	reservations *memory.ReservationRepository
}

func newReservationsEnv(t *testing.T) *reservationsEnv {
	t.Helper()
	log := logger.New("debug", &bytes.Buffer{})
	users := memory.NewUserRepository()
	orders := memory.NewOrderRepository()
	reservations := memory.NewReservationRepository(orders)
	sessions := service.NewSessionService(users, token.NewJWTManager(rbacSecret, rbacIssuer, time.Hour),
		map[string]enum.Role{rbacVendorPhone: enum.RoleVendor}, log)

	r, err := router.New(&bootstrap.Config{}, log, middleware.NewErrorHandler(log), middleware.NewAuthMiddleware(sessions), nil,
		router.Handlers{
			Health:      handler.NewHealthHandler(service.NewHealthService("test", time.Second, log)),
			Auth:        handler.NewAuthHandler(nil, nil),
			Order:       handler.NewOrderHandler(service.NewOrderService(orders)),
			Reservation: handler.NewReservationHandler(service.NewReservationService(reservations)),
		})
	if err != nil {
		t.Fatal(err)
	}
	return &reservationsEnv{r: r, sessions: sessions, orders: orders, reservations: reservations}
}

func (e *reservationsEnv) login(t *testing.T, phone string) *serviceinterface.AuthResult {
	t.Helper()
	res, err := e.sessions.Login(context.Background(), phone)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func (e *reservationsEnv) seedOffer(offerID string, stock, maxPerUser int) {
	lat, lng := 35.70, 51.41
	pickupStart := time.Now().UTC().Add(3 * time.Hour)
	pickupEnd := pickupStart.Add(time.Hour)
	e.reservations.AddOffer(memory.ReservableOffer{
		PublicID:          offerID,
		Title:             "بسته غافلگیری نانوایی",
		OfferType:         "surprise_bag",
		CategoryName:      "نانوایی",
		OriginalPrice:     115000,
		DiscountPrice:     40000,
		ServiceFee:        0,
		QuantityAvailable: stock,
		MaxPerUser:        maxPerUser,
		PickupStart:       pickupStart,
		PickupEnd:         pickupEnd,
		Active:            true,
		Store: model.ReservationStore{
			ID:                 "550e8400-e29b-41d4-a716-446655440000",
			Name:               "نان صبحگاهی",
			City:               "تهران",
			AddressLine:        "خیابان ولیعصر، پلاک ۱۲",
			PickupInstructions: "از پیشخوان تحویل بگیرید.",
			Latitude:           &lat,
			Longitude:          &lng,
		},
	})
}

func TestReservationCheckoutAndIdempotency(t *testing.T) {
	e := newReservationsEnv(t)
	alice := e.login(t, rbacCustomerPhone)
	offerID := "7d0c2a3e-1f5b-4a52-8c64-2f4e9b1a0c33"
	e.seedOffer(offerID, 3, 2)

	// Unauthenticated checkout -> 401
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/reservations", bytes.NewBufferString(`{"offer_id":"`+offerID+`","quantity":1}`)))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated POST /reservations got %d, want 401", w.Code)
	}

	// Create reservation with Idempotency-Key -> 201 Created
	body := `{"offer_id":"` + offerID + `","quantity":2,"terms_accepted":true}`
	req := httptest.NewRequest(http.MethodPost, "/reservations", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+alice.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "checkout-key-1")
	w = httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST /reservations got %d: %s", w.Code, w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("missing Cache-Control: no-store")
	}

	var created dto.ReservationResponse
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.ID == "" || created.Reference == "" || created.PickupCode == "" || created.Status != "paid" || created.TotalAmount != 80000 {
		t.Fatalf("unexpected created reservation: %+v", created)
	}
	if stock, _ := e.reservations.OfferStock(offerID); stock != 1 {
		t.Fatalf("stock after reservation = %d, want 1", stock)
	}

	// Replay with same Idempotency-Key & same payload -> 200 OK, same ID, stock unchanged
	req = httptest.NewRequest(http.MethodPost, "/reservations", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+alice.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "checkout-key-1")
	w = httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || w.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatalf("idempotent replay got %d (header=%q): %s", w.Code, w.Header().Get("Idempotency-Replayed"), w.Body.String())
	}
	var replayed dto.ReservationResponse
	if err := json.Unmarshal(w.Body.Bytes(), &replayed); err != nil {
		t.Fatalf("decode replay: %v", err)
	}
	if replayed.ID != created.ID || replayed.Reference != created.Reference {
		t.Errorf("replay mismatch: got %+v, want %+v", replayed, created)
	}
	if stock, _ := e.reservations.OfferStock(offerID); stock != 1 {
		t.Fatalf("stock changed on replay: got %d, want 1", stock)
	}

	// Reuse same Idempotency-Key with DIFFERENT quantity -> 409 Conflict
	req = httptest.NewRequest(http.MethodPost, "/reservations", bytes.NewBufferString(`{"offer_id":"`+offerID+`","quantity":1}`))
	req.Header.Set("Authorization", "Bearer "+alice.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "checkout-key-1")
	w = httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("idempotency mismatch got %d, want 409: %s", w.Code, w.Body.String())
	}

	// Reservation appears in GET /orders for Alice
	req = httptest.NewRequest(http.MethodGet, "/orders", nil)
	req.Header.Set("Authorization", "Bearer "+alice.AccessToken)
	w = httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /orders got %d: %s", w.Code, w.Body.String())
	}
	var ordersResp dto.OrderListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &ordersResp); err != nil {
		t.Fatalf("decode orders: %v", err)
	}
	if len(ordersResp.Orders) != 1 || ordersResp.Orders[0].ID != created.ID {
		t.Errorf("expected reservation in GET /orders, got %+v", ordersResp.Orders)
	}
}

func TestReservationConfirmationAndOwnership(t *testing.T) {
	e := newReservationsEnv(t)
	alice := e.login(t, rbacCustomerPhone)
	bob := e.login(t, "+989123456700")
	offerID := "7d0c2a3e-1f5b-4a52-8c64-2f4e9b1a0c33"
	e.seedOffer(offerID, 3, 2)

	// Create reservation as Alice
	req := httptest.NewRequest(http.MethodPost, "/reservations", bytes.NewBufferString(`{"offer_id":"`+offerID+`","quantity":2,"terms_accepted":true}`))
	req.Header.Set("Authorization", "Bearer "+alice.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST /reservations got %d: %s", w.Code, w.Body.String())
	}
	var created dto.ReservationResponse
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Unauthenticated -> 401
	w = httptest.NewRecorder()
	e.r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/reservations/"+created.ID, nil))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated GET /reservations/:id got %d, want 401", w.Code)
	}

	// Invalid UUID -> 400
	req = httptest.NewRequest(http.MethodGet, "/reservations/not-a-uuid", nil)
	req.Header.Set("Authorization", "Bearer "+alice.AccessToken)
	w = httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("invalid UUID GET /reservations/:id got %d, want 400", w.Code)
	}

	// Owner gets 200 with persisted confirmation details
	req = httptest.NewRequest(http.MethodGet, "/reservations/"+created.ID, nil)
	req.Header.Set("Authorization", "Bearer "+alice.AccessToken)
	w = httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("GET /reservations/:id got %d: %s", w.Code, w.Body.String())
	}
	var confirmed dto.ReservationResponse
	if err := json.Unmarshal(w.Body.Bytes(), &confirmed); err != nil {
		t.Fatalf("decode confirmation: %v", err)
	}
	if confirmed.ID != created.ID || confirmed.Reference != created.Reference || confirmed.PickupCode != created.PickupCode ||
		confirmed.Status != "paid" || confirmed.Offer.Quantity != 2 || confirmed.TotalAmount != 80000 {
		t.Errorf("confirmation mismatch: %+v", confirmed)
	}

	// Ownership enforcement: Bob cannot read Alice's reservation -> 404
	req = httptest.NewRequest(http.MethodGet, "/reservations/"+created.ID, nil)
	req.Header.Set("Authorization", "Bearer "+bob.AccessToken)
	w = httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("other user GET /reservations/:id got %d, want 404", w.Code)
	}
}

func TestReservationConcurrentCheckoutPreventsOverselling(t *testing.T) {
	e := newReservationsEnv(t)
	offerID := "9d0c2a3e-1f5b-4a52-8c64-2f4e9b1a0c99"
	e.seedOffer(offerID, 3, 2)

	const callers = 10
	tokens := make([]string, callers)
	for i := 0; i < callers; i++ {
		phone := "+9891234561" + string(rune('0'+i/10)) + string(rune('0'+i%10))
		tokens[i] = e.login(t, phone).AccessToken
	}

	var (
		wg        sync.WaitGroup
		created   atomic.Int32
		conflicts atomic.Int32
	)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(tok string) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/reservations", bytes.NewBufferString(`{"offer_id":"`+offerID+`","quantity":1}`))
			req.Header.Set("Authorization", "Bearer "+tok)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			e.r.ServeHTTP(w, req)
			switch w.Code {
			case http.StatusCreated:
				created.Add(1)
			case http.StatusConflict:
				conflicts.Add(1)
			default:
				t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
			}
		}(tokens[i])
	}
	wg.Wait()

	if created.Load() != 3 || conflicts.Load() != callers-3 {
		t.Fatalf("got created=%d conflicts=%d, want created=3 conflicts=%d", created.Load(), conflicts.Load(), callers-3)
	}
	if stock, _ := e.reservations.OfferStock(offerID); stock != 0 {
		t.Fatalf("final stock = %d, want 0", stock)
	}
}
