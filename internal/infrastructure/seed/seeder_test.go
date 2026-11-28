package seed

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tahedig/too-good-to-go/backend/internal/bootstrap"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
)

func TestRun_RefusesProduction(t *testing.T) {
	_, err := Run(context.Background(), nil, Config{
		AppEnv:         bootstrap.EnvProduction,
		AdminPhone:     "+989000000001",
		VendorPhones:   []string{"+989000000002", "+989000000003"},
		CustomerPhones: []string{"+989000000004", "+989000000005"},
	})
	if err == nil {
		t.Fatal("expected error when APP_ENV=production")
	}
}

func TestDefaultSeedDatasetMeetsRequirements(t *testing.T) {
	if len(defaultStores) < 5 {
		t.Fatalf("expected at least 5 stores, got %d", len(defaultStores))
	}
	if len(defaultOffers) < 20 {
		t.Fatalf("expected at least 20 offers, got %d", len(defaultOffers))
	}
	if len(defaultOrders) < 20 {
		t.Fatalf("expected at least 20 orders, got %d", len(defaultOrders))
	}

	storesBySlug := make(map[string]seedStore, len(defaultStores))
	for _, st := range defaultStores {
		if _, dup := storesBySlug[st.Slug]; dup {
			t.Fatalf("duplicate store slug %q", st.Slug)
		}
		storesBySlug[st.Slug] = st
	}

	offersByID := make(map[string]seedOffer, len(defaultOffers))
	for _, off := range defaultOffers {
		if _, ok := storesBySlug[off.StoreSlug]; !ok {
			t.Fatalf("offer %s references unknown store %q", off.PublicID, off.StoreSlug)
		}
		if _, dup := offersByID[off.PublicID]; dup {
			t.Fatalf("duplicate offer public_id %s", off.PublicID)
		}
		pct := (1.0 - float64(off.DiscountPrice)/float64(off.OriginalPrice)) * 100.0
		if pct < 30.0 {
			t.Fatalf("offer %s discount %.1f%% is below 30%% minimum", off.PublicID, pct)
		}
		offersByID[off.PublicID] = off
	}

	seenStatuses := make(map[enum.OrderStatus]bool)
	seenPickupCodes := make(map[string]bool)
	for _, ord := range defaultOrders {
		if _, ok := storesBySlug[ord.StoreSlug]; !ok {
			t.Fatalf("order %s references unknown store %q", ord.PublicID, ord.StoreSlug)
		}
		off, ok := offersByID[ord.OfferPublicID]
		if !ok {
			t.Fatalf("order %s references unknown offer %s", ord.PublicID, ord.OfferPublicID)
		}
		if off.StoreSlug != ord.StoreSlug {
			t.Fatalf("order %s store %q does not match offer store %q", ord.PublicID, ord.StoreSlug, off.StoreSlug)
		}
		if len(ord.PickupCode) != 4 {
			t.Fatalf("order %s pickup code %q must be 4 digits", ord.PublicID, ord.PickupCode)
		}
		key := ord.StoreSlug + ":" + ord.PickupCode
		if seenPickupCodes[key] {
			t.Fatalf("duplicate pickup code %q for store %q", ord.PickupCode, ord.StoreSlug)
		}
		seenPickupCodes[key] = true
		seenStatuses[ord.Status] = true
	}

	required := []enum.OrderStatus{
		enum.OrderStatusPendingPayment,
		enum.OrderStatusPaid,
		enum.OrderStatusReady,
		enum.OrderStatusCompleted,
		enum.OrderStatusCancelled,
		enum.OrderStatusExpired,
	}
	for _, st := range required {
		if !seenStatuses[st] {
			t.Fatalf("missing seeded order for lifecycle status %q", st)
		}
	}

	activePerVendor := make(map[int]int)
	for _, sub := range defaultVendorSubscriptions {
		if sub.Status == "active" {
			activePerVendor[sub.VendorIndex]++
			if activePerVendor[sub.VendorIndex] > 1 {
				t.Fatalf("vendor %d has multiple active subscriptions", sub.VendorIndex)
			}
		}
	}
	for _, pr := range defaultPayoutRequests {
		if _, ok := storesBySlug[pr.StoreSlug]; !ok {
			t.Fatalf("payout request %s references unknown store %q", pr.PublicID, pr.StoreSlug)
		}
		if pr.Amount <= 0 {
			t.Fatalf("payout request %s amount must be positive", pr.PublicID)
		}
	}
	for _, fav := range defaultFavoriteOffers {
		if _, ok := offersByID[fav.OfferPublicID]; !ok {
			t.Fatalf("favorite references unknown offer %s", fav.OfferPublicID)
		}
	}
	if len(defaultStoreRequests) == 0 {
		t.Fatal("expected at least 1 store request")
	}
}

func TestRun_IdempotentIntegration(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping PostgreSQL seed integration test")
	}

	ctx := context.Background()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()

	for _, ext := range []string{"postgis", "pg_trgm"} {
		if _, err := admin.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS "+ext+" SCHEMA public"); err != nil {
			t.Fatalf("extension %s: %v", ext, err)
		}
	}

	schema := fmt.Sprintf("test_seed_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
	}()

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ", public"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	files, err := filepath.Glob(filepath.Join("..", "..", "..", "migrations", "*.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	for _, f := range files {
		sqlBytes, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
			t.Fatalf("apply migration %s: %v", f, err)
		}
	}

	seedCfg := Config{
		AppEnv:         bootstrap.EnvDevelopment,
		AdminPhone:     "+989000000001",
		VendorPhones:   []string{"+989000000002", "+989000000003"},
		CustomerPhones: []string{"+989000000004", "+989000000005"},
	}

	if _, err := Run(ctx, pool, seedCfg); err != nil {
		t.Fatalf("first seed run: %v", err)
	}
	if _, err := Run(ctx, pool, seedCfg); err != nil {
		t.Fatalf("second seed run (idempotency): %v", err)
	}

	checks := []struct {
		query string
		want  int
	}{
		{"SELECT count(*) FROM users", 5},
		{"SELECT count(*) FROM vendors", 2},
		{"SELECT count(*) FROM stores", len(defaultStores)},
		{"SELECT count(*) FROM offers", len(defaultOffers)},
		{"SELECT count(*) FROM orders", len(defaultOrders)},
		{"SELECT count(*) FROM order_items", len(defaultOrders)},
	}
	for _, c := range checks {
		var got int
		if err := pool.QueryRow(ctx, c.query).Scan(&got); err != nil {
			t.Fatalf("query %q: %v", c.query, err)
		}
		if got != c.want {
			t.Fatalf("query %q: got %d, want %d", c.query, got, c.want)
		}
	}
}
