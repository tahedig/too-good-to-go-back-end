package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
	"github.com/tahedig/too-good-to-go/backend/internal/infrastructure/repository/memory"
	"github.com/tahedig/too-good-to-go/backend/pkg/security"
)

const (
	aliceID = "11111111-1111-4111-8111-111111111111"
	bobID   = "22222222-2222-4222-8222-222222222222"
)

// seedOrders gives alice one order per status, an hour apart, oldest first,
// and bob one order. It returns alice's order IDs newest first.
func seedOrders(t *testing.T, repo *memory.OrderRepository) []string {
	t.Helper()
	statuses := []enum.OrderStatus{
		enum.OrderStatusCompleted, enum.OrderStatusCancelled, enum.OrderStatusExpired, enum.OrderStatusNoShow,
		enum.OrderStatusPendingPayment, enum.OrderStatusPaid, enum.OrderStatusReady,
	}
	base := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	var ids []string
	for i, s := range statuses {
		id, _ := security.NewID()
		repo.Add(aliceID, model.Order{ID: id, Status: s, CreatedAt: base.Add(time.Duration(i) * time.Hour)})
		ids = append([]string{id}, ids...)
	}
	bobOrder, _ := security.NewID()
	repo.Add(bobID, model.Order{ID: bobOrder, Status: enum.OrderStatusPaid, CreatedAt: base})
	return ids
}

func ids(page *serviceinterface.OrderPage) []string {
	out := make([]string, len(page.Orders))
	for i, o := range page.Orders {
		out[i] = o.ID
	}
	return out
}

func TestListMyOrdersPaginates(t *testing.T) {
	repo := memory.NewOrderRepository()
	want := seedOrders(t, repo)
	svc := NewOrderService(repo)

	var got []string
	in := serviceinterface.ListOrdersInput{Limit: "3"}
	for pages := 1; ; pages++ {
		page, err := svc.ListMyOrders(context.Background(), aliceID, in)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, ids(page)...)
		if page.NextCursor == "" {
			if pages != 3 {
				t.Fatalf("7 orders in pages of 3 must take 3 pages, took %d", pages)
			}
			break
		}
		in.Cursor = page.NextCursor
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}

	// An exactly full last page has no next cursor.
	page, err := svc.ListMyOrders(context.Background(), aliceID, serviceinterface.ListOrdersInput{Limit: "7"})
	if err != nil || len(page.Orders) != 7 || page.NextCursor != "" {
		t.Fatalf("full page: %d orders, cursor %q, err=%v", len(page.Orders), page.NextCursor, err)
	}
}

func TestListMyOrdersFilters(t *testing.T) {
	repo := memory.NewOrderRepository()
	all := seedOrders(t, repo) // ready, paid, pending_payment, no_show, expired, cancelled, completed
	svc := NewOrderService(repo)

	cases := []struct {
		in   serviceinterface.ListOrdersInput
		want []string
	}{
		{serviceinterface.ListOrdersInput{}, all},
		{serviceinterface.ListOrdersInput{Group: "all"}, all},
		{serviceinterface.ListOrdersInput{Group: "upcoming"}, all[:3]},
		{serviceinterface.ListOrdersInput{Group: "past"}, all[3:]},
		{serviceinterface.ListOrdersInput{Status: "completed, cancelled,completed"}, all[5:]},
		{serviceinterface.ListOrdersInput{Group: "past", Status: "no_show,paid"}, all[3:4]},
		{serviceinterface.ListOrdersInput{Group: "upcoming", Status: "completed"}, nil},
	}
	for _, tc := range cases {
		page, err := svc.ListMyOrders(context.Background(), aliceID, tc.in)
		if err != nil {
			t.Fatalf("%+v: %v", tc.in, err)
		}
		if fmt.Sprint(ids(page)) != fmt.Sprint(tc.want) { // nil and empty both print []
			t.Errorf("%+v: got %v, want %v", tc.in, ids(page), tc.want)
		}
	}
}

func TestListMyOrdersOnlyOwnAndEmpty(t *testing.T) {
	repo := memory.NewOrderRepository()
	seedOrders(t, repo)
	svc := NewOrderService(repo)

	page, err := svc.ListMyOrders(context.Background(), bobID, serviceinterface.ListOrdersInput{})
	if err != nil || len(page.Orders) != 1 {
		t.Fatalf("bob: %v err=%v", ids(page), err)
	}
	page, err = svc.ListMyOrders(context.Background(), "33333333-3333-4333-8333-333333333333", serviceinterface.ListOrdersInput{})
	if err != nil || page.Orders == nil || len(page.Orders) != 0 || page.NextCursor != "" {
		t.Fatalf("empty history must be an empty, non-nil list: %+v err=%v", page, err)
	}
	if _, err := svc.ListMyOrders(context.Background(), "", serviceinterface.ListOrdersInput{}); !isCode(err, exception.CodeUnauthorized) {
		t.Fatalf("no user: got %v, want 401", err)
	}
}

func TestListMyOrdersRejectsBadInput(t *testing.T) {
	svc := NewOrderService(memory.NewOrderRepository())
	notUUID := "MjAyNi0xMS0xM1QxMDowMDowMFp8bm90LWEtdXVpZA" // "2026-11-13T10:00:00Z|not-a-uuid"
	for _, in := range []serviceinterface.ListOrdersInput{
		{Limit: "0"}, {Limit: "51"}, {Limit: "ten"}, {Limit: "-1"},
		{Group: "future"},
		{Status: "shipped"}, {Status: "paid,"},
		{Cursor: "!!!"}, {Cursor: notUUID}, {Cursor: "bm8tc2VwYXJhdG9y"},
	} {
		_, err := svc.ListMyOrders(context.Background(), aliceID, in)
		if !isCode(err, exception.CodeBadRequest) {
			t.Errorf("%+v: got %v, want 400", in, err)
		}
	}
}

func TestOrderCursorRoundTrip(t *testing.T) {
	c := repository.OrderCursor{CreatedAt: time.Date(2026, 11, 13, 10, 0, 0, 123456000, time.UTC), ID: aliceID}
	got, ok := decodeOrderCursor(encodeOrderCursor(c))
	if !ok || !got.CreatedAt.Equal(c.CreatedAt) || got.ID != c.ID {
		t.Fatalf("round trip: %+v ok=%v", got, ok)
	}
}

type failingOrders struct{}

func (failingOrders) ListByUser(context.Context, repository.OrderListQuery) ([]model.Order, error) {
	return nil, errors.New("db down")
}

func (failingOrders) GetActiveStoreByVendorUser(context.Context, string) (*model.VendorStoreActive, error) {
	return nil, errors.New("db down")
}

func (failingOrders) ListVendorOrders(context.Context, string, repository.VendorOrderFilter) ([]model.VendorOrder, int, error) {
	return nil, 0, errors.New("db down")
}

func (failingOrders) GetVendorOrderByID(context.Context, string, string) (*model.VendorOrder, error) {
	return nil, errors.New("db down")
}

func (failingOrders) GetVendorOrderSummary(context.Context, string, string) (*model.VendorOrderSummary, error) {
	return nil, errors.New("db down")
}

func (failingOrders) VerifyPickup(context.Context, string, string, string) (*model.VerifyPickupResult, error) {
	return nil, errors.New("db down")
}

func (failingOrders) UpdateVendorOrderStatus(context.Context, string, string, enum.OrderStatus, string) (*model.VendorOrderStatusUpdateResult, error) {
	return nil, errors.New("db down")
}

func TestListMyOrdersHidesStorageErrors(t *testing.T) {
	_, err := NewOrderService(failingOrders{}).ListMyOrders(context.Background(), aliceID, serviceinterface.ListOrdersInput{})
	if !isCode(err, exception.CodeInternalServer) {
		t.Fatalf("got %v, want 500", err)
	}
}

func TestVendorStore(t *testing.T) {
	repo := memory.NewOrderRepository()
	svc := NewOrderService(repo)

	// Unauthenticated
	if _, err := svc.GetVendorStore(context.Background(), ""); !isCode(err, exception.CodeUnauthorized) {
		t.Fatalf("expected unauthorized, got %v", err)
	}

	// Not found
	if _, err := svc.GetVendorStore(context.Background(), aliceID); !isCode(err, exception.CodeNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}

	// Success
	repo.AddVendorStore(aliceID, model.VendorStoreActive{
		ID:       "store-1",
		Name:     "Test Store",
		Address:  "Tehran",
		Phone:    "09123456789",
		IsActive: true,
	})

	store, err := svc.GetVendorStore(context.Background(), aliceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.ID != "store-1" || store.Name != "Test Store" || !store.IsActive {
		t.Fatalf("unexpected store output: %+v", store)
	}
}

func TestVendorOrdersListAndDetail(t *testing.T) {
	repo := memory.NewOrderRepository()
	svc := NewOrderService(repo)
	vendorID := aliceID

	order1 := model.VendorOrder{
		ID:          "ord-1",
		NumericID:   1,
		OrderNumber: "TG-0001",
		Customer: model.VendorOrderCustomer{
			ID:    "cust-1",
			Name:  "Customer 1",
			Phone: "09120000001",
		},
		Package: model.VendorOrderPackage{
			ID:       "pkg-1",
			Title:    "Bread Bag",
			ImageURL: "https://example.com/bread.jpg",
		},
		Quantity:   2,
		UnitPrice:  40000,
		TotalPrice: 80000,
		Status:     enum.VendorOrderStatusActive,
		PickupDate: "2026-10-19",
		PickupTimeSlot: model.VendorOrderTimeSlot{
			StartTime: "18:00",
			EndTime:   "19:00",
		},
		PickupCode: "1234",
		CreatedAt:  time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC),
	}

	order2 := model.VendorOrder{
		ID:          "ord-2",
		NumericID:   2,
		OrderNumber: "TG-0002",
		Customer: model.VendorOrderCustomer{
			ID:    "cust-2",
			Name:  "Customer 2",
			Phone: "09120000002",
		},
		Package: model.VendorOrderPackage{
			ID:       "pkg-2",
			Title:    "Pastry Box",
			ImageURL: "https://example.com/pastry.jpg",
		},
		Quantity:   1,
		UnitPrice:  60000,
		TotalPrice: 60000,
		Status:     enum.VendorOrderStatusDelivered,
		PickupDate: "2026-10-19",
		PickupTimeSlot: model.VendorOrderTimeSlot{
			StartTime: "17:00",
			EndTime:   "18:00",
		},
		PickupCode: "5678",
		CreatedAt:  time.Date(2026, 10, 19, 11, 0, 0, 0, time.UTC),
	}

	repo.AddVendorOrder(vendorID, order1)
	repo.AddVendorOrder(vendorID, order2)

	// List all
	page, err := svc.ListVendorOrders(context.Background(), vendorID, serviceinterface.VendorOrderListInput{})
	if err != nil {
		t.Fatalf("list error: %v", err)
	}
	if page.TotalItems != 2 || len(page.Orders) != 2 {
		t.Fatalf("expected 2 orders, got total=%d, len=%d", page.TotalItems, len(page.Orders))
	}

	// Filter by status active
	activePage, err := svc.ListVendorOrders(context.Background(), vendorID, serviceinterface.VendorOrderListInput{
		Status: "active",
	})
	if err != nil {
		t.Fatalf("list active error: %v", err)
	}
	if activePage.TotalItems != 1 || activePage.Orders[0].ID != "ord-1" {
		t.Fatalf("expected ord-1, got %+v", activePage.Orders)
	}

	// Filter by date
	datePage, err := svc.ListVendorOrders(context.Background(), vendorID, serviceinterface.VendorOrderListInput{
		Date: "2026-10-19",
	})
	if err != nil {
		t.Fatalf("list date error: %v", err)
	}
	if datePage.TotalItems != 2 {
		t.Fatalf("expected 2 orders on date, got %d", datePage.TotalItems)
	}

	// Get single order
	gotOrder, err := svc.GetVendorOrder(context.Background(), vendorID, "ord-1")
	if err != nil {
		t.Fatalf("get order error: %v", err)
	}
	if gotOrder.ID != "ord-1" || gotOrder.OrderNumber != "TG-0001" {
		t.Fatalf("unexpected order: %+v", gotOrder)
	}

	// Order not found
	if _, err := svc.GetVendorOrder(context.Background(), vendorID, "unknown"); !isCode(err, exception.CodeNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestVendorOrderSummary(t *testing.T) {
	repo := memory.NewOrderRepository()
	svc := NewOrderService(repo)
	vendorID := aliceID

	repo.AddVendorOrder(vendorID, model.VendorOrder{
		ID:         "ord-1",
		Quantity:   2,
		TotalPrice: 80000,
		Status:     enum.VendorOrderStatusActive,
		PickupDate: "2026-10-19",
	})
	repo.AddVendorOrder(vendorID, model.VendorOrder{
		ID:         "ord-2",
		Quantity:   1,
		TotalPrice: 40000,
		Status:     enum.VendorOrderStatusDelivered,
		PickupDate: "2026-10-19",
	})
	repo.AddVendorOrder(vendorID, model.VendorOrder{
		ID:         "ord-3",
		Quantity:   1,
		TotalPrice: 50000,
		Status:     enum.VendorOrderStatusCancelled,
		PickupDate: "2026-10-19",
	})

	summary, err := svc.GetVendorOrderSummary(context.Background(), vendorID, "2026-10-19")
	if err != nil {
		t.Fatalf("summary error: %v", err)
	}
	if summary.TotalReservedPackages != 3 {
		t.Errorf("got TotalReservedPackages=%d, want 3", summary.TotalReservedPackages)
	}
	if summary.ActiveReservationsCount != 1 {
		t.Errorf("got ActiveReservationsCount=%d, want 1", summary.ActiveReservationsCount)
	}
	if summary.DeliveredPackagesCount != 1 {
		t.Errorf("got DeliveredPackagesCount=%d, want 1", summary.DeliveredPackagesCount)
	}
	if summary.TotalRevenue != 120000 {
		t.Errorf("got TotalRevenue=%d, want 120000", summary.TotalRevenue)
	}
}

func TestVerifyPickup(t *testing.T) {
	repo := memory.NewOrderRepository()
	svc := NewOrderService(repo)
	vendorID := aliceID

	repo.AddVendorOrder(vendorID, model.VendorOrder{
		ID:         "ord-1",
		Status:     enum.VendorOrderStatusActive,
		PickupCode: "1234",
	})
	repo.AddVendorOrder(vendorID, model.VendorOrder{
		ID:         "ord-delivered",
		Status:     enum.VendorOrderStatusDelivered,
		PickupCode: "1234",
	})
	repo.AddVendorOrder(vendorID, model.VendorOrder{
		ID:         "ord-cancelled",
		Status:     enum.VendorOrderStatusCancelled,
		PickupCode: "1234",
	})

	// Invalid pickup code
	if _, err := svc.VerifyPickup(context.Background(), vendorID, "ord-1", "9999"); !isCode(err, exception.CodeBadRequest) {
		t.Fatalf("expected bad request for wrong code, got %v", err)
	}

	// Valid verify
	res, err := svc.VerifyPickup(context.Background(), vendorID, "ord-1", "1234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || res.Status != enum.VendorOrderStatusDelivered {
		t.Fatalf("unexpected result: %+v", res)
	}

	// Already delivered
	if _, err := svc.VerifyPickup(context.Background(), vendorID, "ord-delivered", "1234"); !isCode(err, exception.CodeConflict) {
		t.Fatalf("expected conflict for already delivered, got %v", err)
	}

	// Cancelled order cannot be delivered
	if _, err := svc.VerifyPickup(context.Background(), vendorID, "ord-cancelled", "1234"); !isCode(err, exception.CodeBadRequest) {
		t.Fatalf("expected bad request for cancelled order, got %v", err)
	}

	// Order not found
	if _, err := svc.VerifyPickup(context.Background(), vendorID, "non-existent", "1234"); !isCode(err, exception.CodeNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestUpdateVendorOrderStatusAndInventory(t *testing.T) {
	repo := memory.NewOrderRepository()
	svc := NewOrderService(repo)
	vendorID := aliceID
	otherVendorID := bobID

	repo.SetOfferStock("pkg-1", 1)
	repo.AddVendorOrder(vendorID, model.VendorOrder{
		ID:       "ord-paid",
		Status:   enum.VendorOrderStatusActive,
		DBStatus: enum.OrderStatusPaid,
		Package:  model.VendorOrderPackage{ID: "pkg-1", Title: "Bread"},
		Quantity: 2,
	})
	repo.AddVendorOrder(vendorID, model.VendorOrder{
		ID:       "ord-pending",
		Status:   enum.VendorOrderStatusActive,
		DBStatus: enum.OrderStatusPendingPayment,
		Package:  model.VendorOrderPackage{ID: "pkg-1", Title: "Bread"},
		Quantity: 1,
	})
	repo.AddVendorOrder(vendorID, model.VendorOrder{
		ID:       "ord-noshow",
		Status:   enum.VendorOrderStatusReadyForPickup,
		DBStatus: enum.OrderStatusReady,
		Package:  model.VendorOrderPackage{ID: "pkg-1", Title: "Bread"},
		Quantity: 1,
	})

	// Cross-vendor ownership rejected
	if _, err := svc.UpdateVendorOrderStatus(context.Background(), otherVendorID, "ord-paid", "ready_for_pickup", ""); !isCode(err, exception.CodeNotFound) {
		t.Fatalf("expected not found for other vendor, got %v", err)
	}

	// Invalid target status -> 400
	if _, err := svc.UpdateVendorOrderStatus(context.Background(), vendorID, "ord-paid", "delivered", ""); !isCode(err, exception.CodeBadRequest) {
		t.Fatalf("expected bad request for invalid target status, got %v", err)
	}

	// pending_payment cannot transition to ready_for_pickup -> 409
	if _, err := svc.UpdateVendorOrderStatus(context.Background(), vendorID, "ord-pending", "ready_for_pickup", ""); !isCode(err, exception.CodeConflict) {
		t.Fatalf("expected conflict for pending_payment -> ready, got %v", err)
	}

	// paid -> ready_for_pickup succeeds without restoring stock
	res, err := svc.UpdateVendorOrderStatus(context.Background(), vendorID, "ord-paid", "ready_for_pickup", "")
	if err != nil || !res.Success || res.Status != enum.VendorOrderStatusReadyForPickup {
		t.Fatalf("ready transition failed: res=%+v err=%v", res, err)
	}
	if stock := repo.GetOfferStock("pkg-1"); stock != 1 {
		t.Fatalf("expected stock unchanged at 1, got %d", stock)
	}

	// ready_for_pickup -> cancelled restores offer inventory
	res, err = svc.UpdateVendorOrderStatus(context.Background(), vendorID, "ord-paid", "cancelled", "out of stock")
	if err != nil || !res.Success || res.Status != enum.VendorOrderStatusCancelled {
		t.Fatalf("cancel transition failed: res=%+v err=%v", res, err)
	}
	if stock := repo.GetOfferStock("pkg-1"); stock != 3 {
		t.Fatalf("expected stock restored to 3, got %d", stock)
	}

	// cancelled -> ready_for_pickup is invalid -> 409
	if _, err := svc.UpdateVendorOrderStatus(context.Background(), vendorID, "ord-paid", "ready_for_pickup", ""); !isCode(err, exception.CodeConflict) {
		t.Fatalf("expected conflict for cancelled -> ready, got %v", err)
	}

	// ready -> no_show succeeds
	res, err = svc.UpdateVendorOrderStatus(context.Background(), vendorID, "ord-noshow", "no_show", "")
	if err != nil || !res.Success || res.Status != enum.VendorOrderStatusCancelled {
		t.Fatalf("no_show transition failed: res=%+v err=%v", res, err)
	}
}

func isCode(err error, code string) bool {
	var appErr exception.AppError
	return errors.As(err, &appErr) && appErr.Code() == code
}
