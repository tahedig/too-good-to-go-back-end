package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

func TestPostgresAdminDashboardTotalsAndActivity(t *testing.T) {
	pool := newTestPool(t, true)
	ctx := context.Background()
	repo := NewAdminDashboardRepository(pool)

	mustScan := func(sql string, args []any, dst ...any) {
		t.Helper()
		if err := pool.QueryRow(ctx, sql, args...).Scan(dst...); err != nil {
			t.Fatalf("%v\n%s", err, sql)
		}
	}
	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%v\n%s", err, sql)
		}
	}

	var adminID, vendorUserID, cust1ID, cust2ID, deletedCustID int64
	mustScan(`INSERT INTO users (phone, full_name) VALUES ('+989120000201', 'مدیر سامانه') RETURNING id`, nil, &adminID)
	mustScan(`INSERT INTO users (phone, full_name) VALUES ('+989120000202', 'آیدا کریمی') RETURNING id`, nil, &vendorUserID)
	mustScan(`INSERT INTO users (phone, full_name) VALUES ('+989120000203', 'لیلا علیزاده') RETURNING id`, nil, &cust1ID)
	mustScan(`INSERT INTO users (phone, full_name) VALUES ('+989120000204', 'سارا رضایی') RETURNING id`, nil, &cust2ID)
	mustScan(`INSERT INTO users (phone, full_name, status) VALUES ('+989120000205', 'حذف‌شده', 'deleted') RETURNING id`, nil, &deletedCustID)

	for _, uid := range []int64{cust1ID, cust2ID, deletedCustID} {
		mustExec(`INSERT INTO user_roles (user_id, role_id) SELECT $1, id FROM roles WHERE code = 'customer'`, uid)
	}
	var addrID int64
	mustScan(`INSERT INTO user_addresses (user_id, street_name, plaque_number, unit_number) VALUES ($1, 'ولیعصر', '12', '1') RETURNING id`, []any{cust1ID}, &addrID)

	var vendorID int64
	mustScan(`INSERT INTO vendors (owner_user_id, business_name) VALUES ($1, 'نان صبحگاهی') RETURNING id`, []any{vendorUserID}, &vendorID)

	now := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	var approvedStoreID, pendingStoreID int64
	mustScan(`
INSERT INTO stores (vendor_id, name, slug, biography, city, address_line, location, status, accepts_orders, approved_by, approved_at, created_at)
VALUES ($1, 'نان صبحگاهی', 'admin-dash-1', 'نان تازه روزانه', 'تهران', 'خیابان ولیعصر',
        ST_SetSRID(ST_MakePoint(51.41, 35.70), 4326)::geography, 'approved', true, $2, $3, $3)
RETURNING id`, []any{vendorID, adminID, now.Add(-3 * time.Hour)}, &approvedStoreID)

	mustScan(`
INSERT INTO stores (vendor_id, name, slug, biography, city, address_line, location, status, created_at)
VALUES ($1, 'نان صبحگاهی · شعبه شریعتی', 'admin-dash-2', 'شعبه دوم', 'تهران', 'خیابان شریعتی',
        ST_SetSRID(ST_MakePoint(51.44, 35.75), 4326)::geography, 'pending', $2)
RETURNING id`, []any{vendorID, now.Add(-2 * time.Hour)}, &pendingStoreID)

	mustExec(`
INSERT INTO stores (vendor_id, name, slug, biography, city, address_line, location, status, rejection_reason, created_at)
VALUES ($1, 'فروشگاه ردشده', 'admin-dash-3', 'توضیحات فروشگاه', 'تهران', 'انقلاب',
        ST_SetSRID(ST_MakePoint(51.40, 35.70), 4326)::geography, 'rejected', 'نشانی نامعتبر', $2)`,
		vendorID, now.Add(-4*time.Hour))

	mustExec(`
INSERT INTO store_status_history (store_id, from_status, to_status, action, admin_id, created_at)
VALUES ($1, 'pending', 'approved', 'approve', $2, $3)`, approvedStoreID, adminID, now.Add(-90*time.Minute))

	var offerID int64
	mustScan(`
INSERT INTO offers (store_id, offer_type, title, description, original_price, discount_price,
                    quantity_total, quantity_available, pickup_start, pickup_end, allergens_declared, status)
VALUES ($1, 'surprise_bag', 'بسته نان', 'نان و شیرینی تازه روزانه', 100000, 40000,
        5, 3, $2, $3, true, 'active')
RETURNING id`, []any{approvedStoreID, now.Add(time.Hour), now.Add(2 * time.Hour)}, &offerID)

	var orderID int64
	mustScan(`
INSERT INTO orders (user_id, store_id, address_id, status, items_total, service_fee, total_amount,
                    commission_rate, commission_amount, vendor_payout, pickup_code, qr_token,
                    reserved_until, created_at)
VALUES ($1, $2, $3, 'paid', 40000, 0, 40000, 15, 6000, 34000, '1234', 'qr-admin-1', $4, $5)
RETURNING id`, []any{cust1ID, approvedStoreID, addrID, now.Add(10 * time.Minute), now.Add(-30 * time.Minute)}, &orderID)
	mustExec(`
INSERT INTO order_items (order_id, store_id, offer_id, title_snapshot, quantity, unit_original_price, unit_price)
VALUES ($1, $2, $3, 'بسته نان', 1, 100000, 40000)`, orderID, approvedStoreID, offerID)

	dash, err := repo.GetDashboard(ctx, repository.AdminDashboardQuery{
		ActivityLimit: 10,
		Now:           now,
	})
	if err != nil {
		t.Fatalf("GetDashboard: %v", err)
	}
	if dash.Totals.Customers != 2 || dash.Totals.Vendors != 1 || dash.Totals.RegisteredStores != 3 ||
		dash.Totals.ActiveStores != 1 || dash.Totals.ActiveOffers != 1 || dash.Totals.PendingApplications != 1 ||
		dash.Totals.TotalReservations != 1 || dash.Totals.ActiveReservations != 1 ||
		dash.Totals.CompletedReservations != 0 || dash.Totals.PlatformCommission != 6000 {
		t.Fatalf("totals: %+v", dash.Totals)
	}
	if dash.Applications != (model.AdminApplicationBreakdown{Pending: 1, Approved: 1, Rejected: 1, Suspended: 0}) {
		t.Fatalf("applications: %+v", dash.Applications)
	}
	if len(dash.RecentActivity) != 5 || dash.RecentActivity[0].Type != model.AdminActivityReservation {
		t.Fatalf("recent activity: %+v", dash.RecentActivity)
	}

	// Filtering to store_application returns only the 3 store submissions.
	appsOnly, err := repo.GetDashboard(ctx, repository.AdminDashboardQuery{
		ActivityLimit: 10,
		ActivityTypes: []model.AdminActivityType{model.AdminActivityStoreApplication},
		Now:           now,
	})
	if err != nil {
		t.Fatalf("apps-only query: %v", err)
	}
	if len(appsOnly.RecentActivity) != 3 || appsOnly.RecentActivity[0].Title != "نان صبحگاهی · شعبه شریعتی" {
		t.Fatalf("apps-only feed: %+v", appsOnly.RecentActivity)
	}
}
