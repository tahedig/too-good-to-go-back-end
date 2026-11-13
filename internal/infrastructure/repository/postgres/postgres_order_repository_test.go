package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

// orderFixture seeds a store with one offer, and buyers with an address each.
type orderFixture struct {
	t       *testing.T
	pool    *pgxpool.Pool
	storeID int64
	offerID int64
	pickup  time.Time
	codes   int
}

type buyer struct {
	id        int64
	publicID  string
	addressID int64
}

func newOrderFixture(t *testing.T) *orderFixture {
	t.Helper()
	pool := newTestPool(t, true)
	f := &orderFixture{t: t, pool: pool, pickup: time.Date(2026, 11, 13, 14, 30, 0, 0, time.UTC)}
	owner := f.buyer("+989129999999")
	var vendorID int64
	f.scan(`INSERT INTO vendors (owner_user_id, business_name) VALUES ($1, 'Morning Bread Co') RETURNING id`,
		[]any{owner.id}, &vendorID)
	f.scan(`
INSERT INTO stores (vendor_id, name, slug, biography, city, address_line, location)
VALUES ($1, 'نان صبحگاهی', 'morning-bread', 'Fresh bread every day.', 'تهران', 'خیابان ولیعصر، پلاک ۱۲',
        ST_SetSRID(ST_MakePoint(51.41, 35.70), 4326)::geography)
RETURNING id`, []any{vendorID}, &f.storeID)
	f.scan(`
INSERT INTO offers (store_id, offer_type, title, description, original_price, discount_price,
                    quantity_total, quantity_available, pickup_start, pickup_end)
VALUES ($1, 'surprise_bag', 'Bread surprise bag', 'A bag of fresh bread and pastries.', 115000, 40000,
        5, 5, $2, $3)
RETURNING id`, []any{f.storeID, f.pickup, f.pickup.Add(time.Hour)}, &f.offerID)
	return f
}

func (f *orderFixture) scan(sql string, args []any, dst ...any) {
	f.t.Helper()
	if err := f.pool.QueryRow(context.Background(), sql, args...).Scan(dst...); err != nil {
		f.t.Fatalf("%v\n%s", err, sql)
	}
}

func (f *orderFixture) buyer(phone string) buyer {
	f.t.Helper()
	var b buyer
	f.scan(`INSERT INTO users (phone) VALUES ($1) RETURNING id, public_id::text`, []any{phone}, &b.id, &b.publicID)
	f.scan(`INSERT INTO user_addresses (user_id, street_name, plaque_number, unit_number) VALUES ($1, 'Valiasr', '12', '1') RETURNING id`,
		[]any{b.id}, &b.addressID)
	return b
}

// order inserts an order with one item and returns its public ID.
func (f *orderFixture) order(b buyer, status enum.OrderStatus, createdAt time.Time) string {
	f.t.Helper()
	f.codes++
	var id int64
	var publicID string
	f.scan(`
INSERT INTO orders (user_id, store_id, address_id, status, items_total, service_fee, total_amount,
                    commission_rate, commission_amount, vendor_payout, pickup_code, qr_token,
                    reserved_until, created_at)
VALUES ($1, $2, $3, $4, 40000, 0, 40000, 15, 6000, 34000, $5, $6, $7, $8)
RETURNING id, public_id::text`,
		[]any{b.id, f.storeID, b.addressID, string(status), fmt.Sprintf("%04d", f.codes),
			fmt.Sprintf("qr-%d", f.codes), createdAt.Add(10 * time.Minute), createdAt},
		&id, &publicID)
	if _, err := f.pool.Exec(context.Background(), `
INSERT INTO order_items (order_id, store_id, offer_id, title_snapshot, quantity, unit_original_price, unit_price)
VALUES ($1, $2, $3, 'Bread surprise bag', 1, 115000, 40000)`, id, f.storeID, f.offerID); err != nil {
		f.t.Fatal(err)
	}
	return publicID
}

func (f *orderFixture) payment(orderPublicID, status string, createdAt time.Time) {
	f.t.Helper()
	if _, err := f.pool.Exec(context.Background(), `
INSERT INTO payments (order_id, provider, amount, status, authority, created_at)
SELECT id, 'internal_sandbox', 40000, $2::payment_status, $3, $4 FROM orders WHERE public_id = $1::uuid`,
		orderPublicID, status, fmt.Sprintf("auth-%d", createdAt.UnixNano()), createdAt); err != nil {
		f.t.Fatal(err)
	}
}

func listIDs(t *testing.T, repo repository.OrderRepository, q repository.OrderListQuery) []string {
	t.Helper()
	orders, err := repo.ListByUser(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(orders))
	for i, o := range orders {
		ids[i] = o.ID
	}
	return ids
}

func TestPostgresListOrdersFields(t *testing.T) {
	f := newOrderFixture(t)
	repo := NewOrderRepository(f.pool)
	alice := f.buyer("+989121111111")
	created := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	id := f.order(alice, enum.OrderStatusPaid, created)
	f.payment(id, "failed", created.Add(time.Minute))
	f.payment(id, "succeeded", created.Add(2*time.Minute))
	unpaid := f.order(alice, enum.OrderStatusPendingPayment, created.Add(time.Hour))

	orders, err := repo.ListByUser(context.Background(), repository.OrderListQuery{UserID: alice.publicID, Limit: 10})
	if err != nil || len(orders) != 2 {
		t.Fatalf("got %d orders, err=%v", len(orders), err)
	}
	if orders[0].ID != unpaid || orders[0].PaymentStatus != "" || orders[0].Status != enum.OrderStatusPendingPayment {
		t.Fatalf("newest order first, without payment: %+v", orders[0])
	}
	o := orders[1]
	if o.ID != id || o.Status != enum.OrderStatusPaid || o.PaymentStatus != "succeeded" {
		t.Fatalf("status / latest payment: %+v", o)
	}
	if o.TotalAmount != 40000 || o.ItemsTotal != 40000 || o.ServiceFee != 0 || !o.CreatedAt.Equal(created) ||
		!o.ReservedUntil.Equal(created.Add(10*time.Minute)) {
		t.Fatalf("amounts / times: %+v", o)
	}
	if o.Store.Name != "نان صبحگاهی" || o.Store.City != "تهران" || o.Store.AddressLine != "خیابان ولیعصر، پلاک ۱۲" || o.Store.ID == "" {
		t.Fatalf("store: %+v", o.Store)
	}
	if o.PickupStart == nil || !o.PickupStart.Equal(f.pickup) || o.PickupEnd == nil || !o.PickupEnd.Equal(f.pickup.Add(time.Hour)) {
		t.Fatalf("pickup window: %v - %v", o.PickupStart, o.PickupEnd)
	}
	if len(o.Items) != 1 || o.Items[0].Title != "Bread surprise bag" || o.Items[0].OfferType != "surprise_bag" ||
		o.Items[0].Quantity != 1 || o.Items[0].UnitPrice != 40000 {
		t.Fatalf("items: %+v", o.Items)
	}
}

func TestPostgresListOrdersOnlyOwnOrders(t *testing.T) {
	f := newOrderFixture(t)
	repo := NewOrderRepository(f.pool)
	alice, bob := f.buyer("+989121111111"), f.buyer("+989122222222")
	now := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	aliceOrder := f.order(alice, enum.OrderStatusPaid, now)
	bobOrder := f.order(bob, enum.OrderStatusCompleted, now)

	if got := listIDs(t, repo, repository.OrderListQuery{UserID: alice.publicID, Limit: 10}); len(got) != 1 || got[0] != aliceOrder {
		t.Fatalf("alice sees %v", got)
	}
	if got := listIDs(t, repo, repository.OrderListQuery{UserID: bob.publicID, Limit: 10}); len(got) != 1 || got[0] != bobOrder {
		t.Fatalf("bob sees %v", got)
	}
	for _, unknown := range []string{"7c9e6679-7425-40de-944b-e07fc1f90ae7", "not-a-uuid", ""} {
		if got := listIDs(t, repo, repository.OrderListQuery{UserID: unknown, Limit: 10}); len(got) != 0 {
			t.Fatalf("user %q sees %v", unknown, got)
		}
	}
}

func TestPostgresListOrdersFilterAndPaginate(t *testing.T) {
	f := newOrderFixture(t)
	repo := NewOrderRepository(f.pool)
	alice := f.buyer("+989121111111")
	base := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	statuses := []enum.OrderStatus{
		enum.OrderStatusCompleted, enum.OrderStatusCancelled, enum.OrderStatusPaid,
		enum.OrderStatusNoShow, enum.OrderStatusReady,
	}
	var newestFirst []string
	for i, s := range statuses {
		newestFirst = append([]string{f.order(alice, s, base.Add(time.Duration(i)*time.Hour))}, newestFirst...)
	}
	// Two orders created at the same instant must still paginate stably.
	tieA := f.order(alice, enum.OrderStatusExpired, base.Add(10*time.Hour))
	tieB := f.order(alice, enum.OrderStatusExpired, base.Add(10*time.Hour))
	if tieA < tieB {
		tieA, tieB = tieB, tieA
	}
	newestFirst = append([]string{tieA, tieB}, newestFirst...)

	// Walk all pages, two at a time.
	var walked []string
	q := repository.OrderListQuery{UserID: alice.publicID, Limit: 2}
	for page := 0; page < 10; page++ {
		orders, err := repo.ListByUser(context.Background(), q)
		if err != nil {
			t.Fatal(err)
		}
		for _, o := range orders {
			walked = append(walked, o.ID)
		}
		if len(orders) < q.Limit {
			break
		}
		last := orders[len(orders)-1]
		q.After = &repository.OrderCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	if fmt.Sprint(walked) != fmt.Sprint(newestFirst) {
		t.Fatalf("pages:\n got %v\nwant %v", walked, newestFirst)
	}

	upcoming := listIDs(t, repo, repository.OrderListQuery{UserID: alice.publicID, Limit: 10,
		Statuses: enum.OrderGroupUpcoming.Statuses()})
	if fmt.Sprint(upcoming) != fmt.Sprint([]string{newestFirst[2], newestFirst[4]}) {
		t.Fatalf("upcoming: got %v", upcoming)
	}
	if got := listIDs(t, repo, repository.OrderListQuery{UserID: alice.publicID, Limit: 10,
		Statuses: []enum.OrderStatus{}}); len(got) != 0 {
		t.Fatalf("an empty status list must match nothing, got %v", got)
	}
}
