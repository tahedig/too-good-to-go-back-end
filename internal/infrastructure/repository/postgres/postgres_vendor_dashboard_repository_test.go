package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

func TestPostgresVendorDashboardOwnershipAndMetrics(t *testing.T) {
	pool := newTestPool(t, true)
	ctx := context.Background()
	repo := NewVendorDashboardRepository(pool)

	var (
		adminID, vendorAUserID, vendorBUserID, customerID int64
		vendorAUserPub, vendorBUserPub                    string
		vendorAID, vendorBID                              int64
		storeA1ID, storeA2ID, storeBID                    int64
		storeA1Pub, storeA2Pub, storeBPub                 string
		addressID                                         int64
		offerA1ID, offerBID                               int64
	)
	mustScan := func(sql string, args []any, dst ...any) {
		t.Helper()
		if err := pool.QueryRow(ctx, sql, args...).Scan(dst...); err != nil {
			t.Fatalf("%v\n%s", err, sql)
		}
	}

	mustScan(`INSERT INTO users (phone, full_name) VALUES ('+989120000101', 'مدیر') RETURNING id`, nil, &adminID)
	mustScan(`INSERT INTO users (phone, full_name) VALUES ('+989120000102', 'فروشنده اول') RETURNING id, public_id::text`, nil, &vendorAUserID, &vendorAUserPub)
	mustScan(`INSERT INTO users (phone, full_name) VALUES ('+989120000103', 'فروشنده دوم') RETURNING id, public_id::text`, nil, &vendorBUserID, &vendorBUserPub)
	mustScan(`INSERT INTO users (phone, full_name) VALUES ('+989120000104', 'لیلا علیزاده') RETURNING id`, nil, &customerID)
	mustScan(`INSERT INTO user_addresses (user_id, street_name, plaque_number, unit_number) VALUES ($1, 'ولیعصر', '12', '1') RETURNING id`, []any{customerID}, &addressID)

	mustScan(`INSERT INTO vendors (owner_user_id, business_name, commission_rate) VALUES ($1, 'نان صبحگاهی', 10) RETURNING id`, []any{vendorAUserID}, &vendorAID)
	mustScan(`INSERT INTO vendors (owner_user_id, business_name) VALUES ($1, 'فروشگاه رقیب') RETURNING id`, []any{vendorBUserID}, &vendorBID)

	now := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	mustScan(`
INSERT INTO stores (vendor_id, name, slug, biography, city, address_line, location, status, approved_by, approved_at, created_at)
VALUES ($1, 'نان صبحگاهی', 'morning-bread-1', 'نان تازه روزانه', 'تهران', 'خیابان ولیعصر، پلاک ۱۲',
        ST_SetSRID(ST_MakePoint(51.41, 35.70), 4326)::geography, 'approved', $2, $3, $3)
RETURNING id, public_id::text`, []any{vendorAID, adminID, now.Add(-48 * time.Hour)}, &storeA1ID, &storeA1Pub)

	mustScan(`
INSERT INTO stores (vendor_id, name, slug, biography, city, address_line, location, status, created_at)
VALUES ($1, 'نان صبحگاهی · شعبه شریعتی', 'morning-bread-2', 'شعبه دوم', 'تهران', 'خیابان شریعتی، پلاک ۲۴',
        ST_SetSRID(ST_MakePoint(51.44, 35.75), 4326)::geography, 'pending', $2)
RETURNING id, public_id::text`, []any{vendorAID, now.Add(-12 * time.Hour)}, &storeA2ID, &storeA2Pub)

	mustScan(`
INSERT INTO stores (vendor_id, name, slug, biography, city, address_line, location, status, approved_by, approved_at, created_at)
VALUES ($1, 'رقیب', 'competitor-1', 'فروشگاه رقیب', 'تهران', 'میدان ونک',
        ST_SetSRID(ST_MakePoint(51.40, 35.76), 4326)::geography, 'approved', $2, $3, $3)
RETURNING id, public_id::text`, []any{vendorBID, adminID, now.Add(-48 * time.Hour)}, &storeBID, &storeBPub)

	mustScan(`
INSERT INTO offers (store_id, offer_type, title, description, original_price, discount_price,
                    quantity_total, quantity_available, pickup_start, pickup_end, allergens_declared, status)
VALUES ($1, 'surprise_bag', 'بسته غافلگیری نانوایی', 'نان و شیرینی تازه پایان روز', 100000, 40000,
        6, 3, $2, $3, true, 'active')
RETURNING id`, []any{storeA1ID, now.Add(2 * time.Hour), now.Add(3 * time.Hour)}, &offerA1ID)

	mustScan(`
INSERT INTO offers (store_id, offer_type, title, description, original_price, discount_price,
                    quantity_total, quantity_available, pickup_start, pickup_end, allergens_declared, status)
VALUES ($1, 'surprise_bag', 'بسته رقیب', 'بسته غذای فروشگاه رقیب', 100000, 50000,
        10, 8, $2, $3, true, 'active')
RETURNING id`, []any{storeBID, now.Add(time.Hour), now.Add(2 * time.Hour)}, &offerBID)

	insertOrder := func(storeID, offerID int64, status enum.OrderStatus, qty int, total, comm, payout int64, code, qr string, createdAt time.Time, completedAt *time.Time) string {
		var orderID int64
		var pub string
		mustScan(`
INSERT INTO orders (user_id, store_id, address_id, status, items_total, service_fee, total_amount,
                    commission_rate, commission_amount, vendor_payout, pickup_code, qr_token,
                    reserved_until, completed_at, created_at)
VALUES ($1, $2, $3, $4::order_status, $5, 0, $5, 10, $6, $7, $8, $9, $10, $11, $12)
RETURNING id, public_id::text`,
			[]any{customerID, storeID, addressID, string(status), total, comm, payout, code, qr, createdAt.Add(10 * time.Minute), completedAt, createdAt},
			&orderID, &pub)
		if _, err := pool.Exec(ctx, `
INSERT INTO order_items (order_id, store_id, offer_id, title_snapshot, quantity, unit_original_price, unit_price)
VALUES ($1, $2, $3, 'بسته غافلگیری نانوایی', $4, 100000, 40000)`, orderID, storeID, offerID, qty); err != nil {
			t.Fatal(err)
		}
		return pub
	}

	paidTodayPub := insertOrder(storeA1ID, offerA1ID, enum.OrderStatusPaid, 1, 40000, 4000, 36000, "1001", "qr-a1", now.Add(-30*time.Minute), nil)
	completedYesterday := now.Add(-26 * time.Hour)
	insertOrder(storeA1ID, offerA1ID, enum.OrderStatusCompleted, 2, 80000, 8000, 72000, "1002", "qr-a2", completedYesterday, &completedYesterday)
	insertOrder(storeBID, offerBID, enum.OrderStatusPaid, 4, 200000, 30000, 170000, "2001", "qr-b1", now.Add(-10*time.Minute), nil)

	dash, err := repo.GetDashboard(ctx, repository.VendorDashboardQuery{
		VendorUserID: vendorAUserPub,
		RecentLimit:  10,
		Now:          now,
	})
	if err != nil {
		t.Fatalf("GetDashboard: %v", err)
	}
	if dash.ActiveStore == nil || dash.ActiveStore.ID != storeA1Pub || len(dash.Stores) != 2 {
		t.Fatalf("stores: active=%+v all=%+v", dash.ActiveStore, dash.Stores)
	}
	if len(dash.PendingStoreRequests) != 1 || dash.PendingStoreRequests[0].ID != storeA2Pub {
		t.Fatalf("pending banners: %+v", dash.PendingStoreRequests)
	}
	if dash.KPIs.ActiveOffers != 1 || dash.KPIs.AvailablePackages != 3 ||
		dash.KPIs.ReservedPackages != 1 || dash.KPIs.ReceivedRevenue != 72000 {
		t.Fatalf("kpis: %+v", dash.KPIs)
	}
	if dash.ReservationCounts.Paid != 1 || dash.ReservationCounts.Completed != 1 ||
		dash.ReservationCounts.Active != 1 || dash.ReservationCounts.Total != 2 {
		t.Fatalf("counts: %+v", dash.ReservationCounts)
	}
	if dash.Today.ReservationsCount != 1 || dash.Today.PackagesReserved != 1 ||
		dash.Today.CompletedCount != 0 || dash.Today.Revenue != 36000 {
		t.Fatalf("today: %+v", dash.Today)
	}
	if dash.Financial.ReceivedPayout != 72000 || dash.Financial.PendingPayout != 36000 ||
		dash.Financial.NetPeriodRevenue != 108000 || dash.Financial.Plan.CommissionRate != 10 {
		t.Fatalf("financial: %+v", dash.Financial)
	}
	if dash.NearestReservation == nil || dash.NearestReservation.ID != paidTodayPub ||
		dash.NearestReservation.OfferQuantityAvailable != 3 || dash.NearestReservation.OfferQuantityReserved != 3 ||
		dash.NearestReservation.OfferQuantityTotal != 6 {
		t.Fatalf("nearest: %+v", dash.NearestReservation)
	}
	if len(dash.RecentReservations) != 2 || dash.RecentReservations[0].ID != paidTodayPub {
		t.Fatalf("recent: %+v", dash.RecentReservations)
	}

	if _, err := repo.GetDashboard(ctx, repository.VendorDashboardQuery{
		VendorUserID: vendorAUserPub,
		StoreID:      storeBPub,
		RecentLimit:  10,
		Now:          now,
	}); !errors.Is(err, repository.ErrStoreNotFound) {
		t.Fatalf("cross-vendor store filter: got %v, want ErrStoreNotFound", err)
	}

	dashB, err := repo.GetDashboard(ctx, repository.VendorDashboardQuery{
		VendorUserID: vendorBUserPub,
		RecentLimit:  10,
		Now:          now,
	})
	if err != nil {
		t.Fatalf("vendor B dashboard: %v", err)
	}
	if dashB.ReservationCounts.Total != 1 || dashB.Financial.Plan.CommissionRate != 15 {
		t.Fatalf("vendor B metrics: %+v", dashB)
	}
}
