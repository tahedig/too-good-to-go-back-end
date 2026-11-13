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

func TestListMyOrdersHidesStorageErrors(t *testing.T) {
	_, err := NewOrderService(failingOrders{}).ListMyOrders(context.Background(), aliceID, serviceinterface.ListOrdersInput{})
	if !isCode(err, exception.CodeInternalServer) {
		t.Fatalf("got %v, want 500", err)
	}
}

func isCode(err error, code string) bool {
	var appErr exception.AppError
	return errors.As(err, &appErr) && appErr.Code() == code
}
