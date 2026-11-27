package postgres

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

func newAdminStoreTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping PostgreSQL integration test")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)

	for _, ext := range []string{"postgis", "pg_trgm"} {
		if _, err := admin.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS "+ext+" SCHEMA public"); err != nil {
			t.Fatalf("extension %s: %v", ext, err)
		}
	}
	schema := fmt.Sprintf("test_admin_store_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") })

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ", public"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	migrations := []string{
		"000001_create_roles.up.sql",
		"000002_create_users.up.sql",
		"000003_create_user_roles.up.sql",
		"000004_create_otp_codes.up.sql",
		"000005_create_platform_settings.up.sql",
		"000006_create_user_addresses.up.sql",
		"000007_create_stores.up.sql",
		"000008_create_offers.up.sql",
		"000009_create_orders.up.sql",
		"000010_create_payments.up.sql",
		"000011_create_store_profile_details.up.sql",
		"000012_create_store_requests.up.sql",
		"000013_create_store_status_history.up.sql",
		"000014_reservation_checkout_support.up.sql",
		"000015_create_catalogue_views_and_favorites.up.sql",
		"000016_create_financial_management.up.sql",
	}
	for _, f := range migrations {
		sqlBytes, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "migrations", f))
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
			t.Fatalf("apply %s: %v", f, err)
		}
	}
	return pool
}

func TestAdminStoreRepository_Postgres(t *testing.T) {
	pool := newAdminStoreTestPool(t)
	ctx := context.Background()
	repo := NewAdminStoreRepository(pool)

	var adminID, ownerID, customerID, vendorID, storeDBID int64
	var storePublicID string

	if err := pool.QueryRow(ctx, `INSERT INTO users (phone, full_name) VALUES ('+989120000011', 'مدیر') RETURNING id`).Scan(&adminID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO users (phone, full_name) VALUES ('+989120000012', 'آیدا کریمی') RETURNING id`).Scan(&ownerID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO users (phone, full_name) VALUES ('+989120000013', 'لیلا علیزاده') RETURNING id`).Scan(&customerID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO vendors (owner_user_id, business_name, contact_phone) VALUES ($1, 'نان صبحگاهی', '09125550120') RETURNING id`, ownerID).Scan(&vendorID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
INSERT INTO stores (vendor_id, category_id, name, slug, biography, city, district, address_line, location, status, approved_by, approved_at)
VALUES ($1, 3, 'نان صبحگاهی', 'nan-sobhgahi', 'نانوایی سنگک', 'تهران', 'ولیعصر', 'خیابان ولیعصر، پلاک ۱۲',
        ST_SetSRID(ST_MakePoint(51.3347, 35.7219), 4326)::geography, 'approved', $2, now())
RETURNING id, public_id::text`, vendorID, adminID).Scan(&storeDBID, &storePublicID); err != nil {
		t.Fatal(err)
	}

	if _, err := pool.Exec(ctx, `
INSERT INTO store_working_hours (store_id, day_of_week, open_time, close_time, is_closed)
VALUES ($1, 0, '08:00', '19:00', false), ($1, 5, '00:00', '00:00', true)`, storeDBID); err != nil {
		t.Fatal(err)
	}

	// 1. GetStore & UpdateStoreStatus
	detail, err := repo.GetStore(ctx, storePublicID)
	if err != nil {
		t.Fatalf("GetStore: %v", err)
	}
	if !detail.IsActive || detail.Name != "نان صبحگاهی" || len(detail.WorkingHours) != 2 {
		t.Fatalf("unexpected detail: %+v", detail)
	}
	updatedDetail, err := repo.UpdateStoreStatus(ctx, storePublicID, false)
	if err != nil || updatedDetail.IsActive {
		t.Fatalf("UpdateStoreStatus(false): err=%v, is_active=%v", err, updatedDetail.IsActive)
	}

	// 2. GetStoreSubscription
	if _, err := pool.Exec(ctx, `
INSERT INTO vendor_subscriptions (vendor_id, plan_id, status, starts_at, ends_at)
VALUES ($1, (SELECT id FROM subscription_plans WHERE code = 'basic'),
        'active', now() - interval '27 days', now() + interval '3 days'),
       ($1, (SELECT id FROM subscription_plans WHERE code = 'pro'), 'pending_review', NULL, NULL)`, vendorID); err != nil {
		t.Fatal(err)
	}
	sub, err := repo.GetStoreSubscription(ctx, storePublicID)
	if err != nil {
		t.Fatalf("GetStoreSubscription: %v", err)
	}
	if sub.Status != "pending_review" || sub.CurrentPlan != "پایه" || sub.UpgradeRequest == nil || *sub.UpgradeRequest != "حرفه‌ای" || sub.RemainingDays < 2 {
		t.Fatalf("unexpected subscription: %+v", sub)
	}

	// 3. Offers
	var offerDBID int64
	var offerPublicID string
	if err := pool.QueryRow(ctx, `
INSERT INTO offers (store_id, offer_type, title, description, category_id, original_price, discount_price,
                    quantity_total, quantity_available, pickup_start, pickup_end, status, allergens_declared)
VALUES ($1, 'surprise_bag', 'بسته غافلگیری نانوایی', 'نان تازه و گرم پایان روز', 3, 115000, 40000,
        5, 3, now() + interval '1 hour', now() + interval '4 hours', 'active', true)
RETURNING id, public_id::text`, storeDBID).Scan(&offerDBID, &offerPublicID); err != nil {
		t.Fatal(err)
	}

	offers, err := repo.ListStoreOffers(ctx, storePublicID)
	if err != nil || len(offers) != 1 || offers[0].DiscountPrice != 40000 {
		t.Fatalf("ListStoreOffers: err=%v, offers=%+v", err, offers)
	}
	offer, err := repo.GetOffer(ctx, offerPublicID)
	if err != nil || !offer.IsActive {
		t.Fatalf("GetOffer: err=%v, offer=%+v", err, offer)
	}
	deactivatedOffer, err := repo.UpdateOfferStatus(ctx, offerPublicID, false)
	if err != nil || deactivatedOffer.IsActive || deactivatedOffer.Status != "cancelled" {
		t.Fatalf("UpdateOfferStatus: err=%v, offer=%+v", err, deactivatedOffer)
	}

	// 4. Orders
	var addrID, orderID int64
	if err := pool.QueryRow(ctx, `
INSERT INTO user_addresses (user_id, label, street_name, plaque_number, unit_number)
VALUES ($1, 'خانه', 'خیابان ولیعصر', '۱۲', '۱')
RETURNING id`, customerID).Scan(&addrID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
INSERT INTO orders (user_id, store_id, address_id, status, items_total, service_fee, total_amount,
                    commission_rate, commission_amount, vendor_payout, pickup_code, qr_token, reserved_until)
VALUES ($1, $2, $3, 'paid', 40000, 0, 40000, 10.00, 4000, 36000, '1234', 'qr-test-1', $4)
RETURNING id`, customerID, storeDBID, addrID, time.Now().Add(time.Hour)).Scan(&orderID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO order_items (order_id, store_id, offer_id, title_snapshot, quantity, unit_original_price, unit_price)
VALUES ($1, $2, $3, 'بسته غافلگیری نانوایی', 1, 115000, 40000)`, orderID, storeDBID, offerDBID); err != nil {
		t.Fatal(err)
	}

	orderPage, err := repo.ListStoreOrders(ctx, storePublicID, repository.AdminStoreOrderQuery{
		Statuses: []string{"pending_payment", "paid", "ready"},
		Page:     1,
		PageSize: 20,
	})
	if err != nil || orderPage.Total != 1 || orderPage.Items[0].CustomerName != "لیلا علیزاده" || orderPage.Items[0].Quantity != 1 {
		t.Fatalf("ListStoreOrders: err=%v, page=%+v", err, orderPage)
	}
}
