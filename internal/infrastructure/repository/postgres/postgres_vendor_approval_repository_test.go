package postgres

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

const decisionMessage = "نشانی تحویل قابل تأیید نبود. لطفاً نشانی را اصلاح کنید."

type vendorFixture struct {
	t        *testing.T
	pool     *pgxpool.Pool
	adminID  string // public ID
	vendorID int64
	n        int
}

func newVendorFixture(t *testing.T) *vendorFixture {
	t.Helper()
	f := &vendorFixture{t: t, pool: newTestPool(t, true)}
	var adminDBID, ownerID int64
	f.scanRow(`INSERT INTO users (phone, full_name) VALUES ('+989120000001', 'مدیر سامانه') RETURNING id, public_id::text`,
		nil, &adminDBID, &f.adminID)
	f.scanRow(`INSERT INTO users (phone, full_name) VALUES ('+989120000002', 'سمیرا محمدی') RETURNING id, public_id::text`,
		nil, &ownerID, new(string))
	f.scanRow(`INSERT INTO vendors (owner_user_id, business_name, contact_phone) VALUES ($1, 'Samira Foods', '+989120000003') RETURNING id`,
		[]any{ownerID}, &f.vendorID)
	return f
}

func (f *vendorFixture) scanRow(sql string, args []any, dst ...any) {
	f.t.Helper()
	if err := f.pool.QueryRow(context.Background(), sql, args...).Scan(dst...); err != nil {
		f.t.Fatalf("%v\n%s", err, sql)
	}
}

// store inserts a store in the given status, submitted n minutes after a
// fixed time so the list order is predictable. It returns its public ID.
func (f *vendorFixture) store(name, address string, status enum.StoreStatus) string {
	f.t.Helper()
	f.n++
	var id string
	f.scanRow(`
INSERT INTO stores (vendor_id, category_id, name, slug, biography, city, address_line, location, status,
                    rejection_reason, approved_by, approved_at, created_at)
VALUES ($1, 3, $2, $3, 'Bread and pastries of the day.', 'تهران', $4,
        ST_SetSRID(ST_MakePoint(51.41, 35.70), 4326)::geography, $5::store_status,
        CASE WHEN $5 = 'rejected' THEN 'seeded' END,
        CASE WHEN $5 IN ('approved', 'suspended') THEN (SELECT id FROM users WHERE phone = '+989120000001') END,
        CASE WHEN $5 IN ('approved', 'suspended') THEN now() END,
        $6)
RETURNING public_id::text`,
		[]any{f.vendorID, name, fmt.Sprintf("store-%d", f.n), address, string(status),
			time.Date(2026, 10, 1, 0, f.n, 0, 0, time.UTC)}, &id)
	return id
}

func (f *vendorFixture) transition(repo repository.VendorApprovalRepository, storeID string, action enum.StoreAction) (*model.VendorStore, error) {
	from, to, _ := action.Transition()
	t := model.StoreTransition{StoreID: storeID, Action: action, From: from, To: to, AdminUserID: f.adminID}
	if action.NeedsReason() {
		t.ReasonCode, t.Message = action.ReasonCodes()[0], decisionMessage
	}
	return repo.Transition(context.Background(), t)
}

func (f *vendorFixture) historyCount(storeID string) int {
	f.t.Helper()
	var n int
	f.scanRow(`SELECT count(*) FROM store_status_history h JOIN stores s ON s.id = h.store_id
WHERE s.public_id = $1::uuid AND h.admin_id = (SELECT id FROM users WHERE phone = '+989120000001')`, []any{storeID}, &n)
	return n
}

func TestPostgresVendorDecisionFlow(t *testing.T) {
	f := newVendorFixture(t)
	repo := NewVendorApprovalRepository(f.pool)
	ctx := context.Background()
	store := f.store("نانوایی سنتی", "خیابان شریعتی، پلاک ۲۴", enum.StoreStatusPending)

	got, err := repo.Get(ctx, store)
	if err != nil || got.Status != enum.StoreStatusPending || got.OwnerName != "سمیرا محمدی" || got.Category != "نانوایی" ||
		got.Phone != "+989120000003" || got.StatusUpdatedAt != nil || got.Decision != nil {
		t.Fatalf("pending store: %+v err=%v", got, err)
	}

	got, err = f.transition(repo, store, enum.StoreActionApprove)
	if err != nil || got.Status != enum.StoreStatusApproved || got.StatusUpdatedAt == nil || got.Decision != nil {
		t.Fatalf("approve: %+v err=%v", got, err)
	}
	var approvedByAdmin bool
	f.scanRow(`SELECT approved_by = (SELECT id FROM users WHERE phone = '+989120000001') AND approved_at IS NOT NULL
FROM stores WHERE public_id = $1::uuid`, []any{store}, &approvedByAdmin)
	if !approvedByAdmin {
		t.Fatal("approve must record the approving admin")
	}

	got, err = f.transition(repo, store, enum.StoreActionSuspend)
	if err != nil || got.Status != enum.StoreStatusSuspended || got.Decision == nil ||
		got.Decision.Action != enum.StoreActionSuspend || got.Decision.Message != decisionMessage ||
		got.Decision.DecidedBy != "مدیر سامانه" {
		t.Fatalf("suspend: %+v err=%v", got, err)
	}

	// One source of truth: the publish trigger reads the same stores.status.
	_, err = f.pool.Exec(ctx, `
INSERT INTO offers (store_id, offer_type, title, description, original_price, discount_price, quantity_total,
                    quantity_available, pickup_start, pickup_end, allergens_declared, status)
SELECT id, 'surprise_bag', 'Bag', 'Bread of the day for you.', 100000, 40000, 3, 3, now(), now() + interval '1 hour', true, 'active'
FROM stores WHERE public_id = $1::uuid`, store)
	if err == nil {
		t.Fatal("a suspended store must not be able to publish an offer")
	}

	got, err = f.transition(repo, store, enum.StoreActionReactivate)
	if err != nil || got.Status != enum.StoreStatusApproved || got.Decision != nil {
		t.Fatalf("reactivate: %+v err=%v", got, err)
	}
	if n := f.historyCount(store); n != 3 {
		t.Fatalf("history rows: %d, want 3", n)
	}

	rejected := f.store("Shop to reject", "خیابان انقلاب", enum.StoreStatusPending)
	got, err = f.transition(repo, rejected, enum.StoreActionReject)
	if err != nil || got.Status != enum.StoreStatusRejected || got.Decision == nil || got.Decision.Action != enum.StoreActionReject ||
		got.Decision.ReasonCode != "unverifiable_address" {
		t.Fatalf("reject: %+v err=%v", got, err)
	}
	var reason string
	f.scanRow(`SELECT rejection_reason FROM stores WHERE public_id = $1::uuid`, []any{rejected}, &reason)
	if reason != decisionMessage {
		t.Fatalf("rejection_reason = %q", reason)
	}
}

func TestPostgresVendorTransitionConflicts(t *testing.T) {
	f := newVendorFixture(t)
	repo := NewVendorApprovalRepository(f.pool)
	store := f.store("Bakery", "Valiasr", enum.StoreStatusPending)

	if _, err := f.transition(repo, store, enum.StoreActionSuspend); !errors.Is(err, repository.ErrStoreStatusChanged) {
		t.Fatalf("suspend pending: %v", err)
	}
	for _, id := range []string{"7c9e6679-7425-40de-944b-e07fc1f90ae7", "not-a-uuid"} {
		if _, err := f.transition(repo, id, enum.StoreActionApprove); !errors.Is(err, repository.ErrStoreNotFound) {
			t.Fatalf("%s: %v", id, err)
		}
		if _, err := repo.Get(context.Background(), id); !errors.Is(err, repository.ErrStoreNotFound) {
			t.Fatalf("get %s: %v", id, err)
		}
	}

	// Ten admins approve at once: exactly one wins, one history row.
	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		wins      int
		conflicts int
	)
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := f.transition(repo, store, enum.StoreActionApprove)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				wins++
			case errors.Is(err, repository.ErrStoreStatusChanged):
				conflicts++
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()
	if wins != 1 || conflicts != 9 || f.historyCount(store) != 1 {
		t.Fatalf("wins=%d conflicts=%d history=%d", wins, conflicts, f.historyCount(store))
	}
}

func TestPostgresVendorList(t *testing.T) {
	f := newVendorFixture(t)
	repo := NewVendorApprovalRepository(f.pool)
	ctx := context.Background()
	ids := []string{
		f.store("نانوایی سنتی", "خیابان شریعتی", enum.StoreStatusPending),
		f.store("Cafe ROSE", "خیابان انقلاب", enum.StoreStatusApproved),
		f.store("100% Fresh", "خیابان آزادی", enum.StoreStatusApproved),
		f.store("Fruit Shop", "خیابان ولیعصر", enum.StoreStatusRejected),
		f.store("Pastry", "میدان ونک", enum.StoreStatusSuspended),
	}
	all := []string{ids[4], ids[3], ids[2], ids[1], ids[0]} // newest first
	wantCounts := map[enum.StoreStatus]int{"pending": 1, "approved": 2, "rejected": 1, "suspended": 1}

	page, err := repo.List(ctx, repository.VendorStoreQuery{Limit: 2})
	if err != nil || page.Total != 5 || len(page.Items) != 2 || page.Items[0].ID != ids[4] || page.Items[1].ID != ids[3] ||
		fmt.Sprint(page.Counts) != fmt.Sprint(wantCounts) {
		t.Fatalf("first page: %+v err=%v", page, err)
	}

	cases := []struct {
		q    repository.VendorStoreQuery
		want []string
	}{
		{repository.VendorStoreQuery{Statuses: []enum.StoreStatus{"approved", "pending"}}, []string{ids[2], ids[1], ids[0]}},
		{repository.VendorStoreQuery{Search: "rose"}, []string{ids[1]}}, // case-insensitive
		{repository.VendorStoreQuery{Search: "سنتی"}, []string{ids[0]}}, // Persian
		{repository.VendorStoreQuery{Search: "ونک"}, []string{ids[4]}},  // address
		{repository.VendorStoreQuery{Search: "سمیرا"}, all},             // owner name, shared by all
		{repository.VendorStoreQuery{Search: "%"}, []string{ids[2]}},    // % is literal
		{repository.VendorStoreQuery{Search: "نانوایی"}, all},           // category, shared by all
		{repository.VendorStoreQuery{Statuses: []enum.StoreStatus{"approved"}, Search: "fresh"}, []string{ids[2]}},
		{repository.VendorStoreQuery{Offset: 10}, []string{}},
	}
	for i, tc := range cases {
		tc.q.Limit = 20
		page, err := repo.List(ctx, tc.q)
		if err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		var got []string
		for _, s := range page.Items {
			got = append(got, s.ID)
		}
		if fmt.Sprint(got) != fmt.Sprint(tc.want) {
			t.Errorf("case %d %+v: got %v, want %v", i, tc.q, got, tc.want)
		}
		if tc.q.Offset == 0 && page.Total != len(tc.want) {
			t.Errorf("case %d: total %d, want %d", i, page.Total, len(tc.want))
		}
		if fmt.Sprint(page.Counts) != fmt.Sprint(wantCounts) {
			t.Errorf("case %d: counts must ignore filters: %v", i, page.Counts)
		}
	}
}
