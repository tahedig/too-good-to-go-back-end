package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

func TestPostgresStoreRepositoryAdminListAndUpdate(t *testing.T) {
	f := newVendorFixture(t)
	repo := NewAdminStoreManagementRepository(f.pool)
	ctx := context.Background()

	firstID := f.store("نانوایی سنتی", "خیابان شریعتی", enum.StoreStatusApproved)
	secondID := f.store("نان شیرمال", "خیابان انقلاب", enum.StoreStatusApproved)
	deletedID := f.store("نان حذف شده", "خیابان ولیعصر", enum.StoreStatusApproved)
	if _, err := f.pool.Exec(ctx, `
UPDATE stores SET owner_name = CASE public_id
    WHEN $1::uuid THEN 'آیدا کریمی'
    WHEN $2::uuid THEN 'سمیرا محمدی'
    ELSE 'مالک حذف شده'
END,
accepts_orders = (public_id = $1::uuid),
deleted_at = CASE WHEN public_id = $3::uuid THEN now() END
WHERE public_id = ANY(ARRAY[$1::uuid, $2::uuid, $3::uuid])`, firstID, secondID, deletedID); err != nil {
		t.Fatal(err)
	}

	trueValue := true
	category := int16(3)
	searchPage, err := repo.ListAdmin(ctx, repository.AdminStoreFilter{
		Search: "نان", Status: string(enum.StoreStatusApproved), CategoryID: &category,
		City: "تهران", AcceptsOrders: &trueValue, Sort: "newest", Limit: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if searchPage.Total != 1 || len(searchPage.Items) != 1 || searchPage.Items[0].PublicID != firstID {
		t.Fatalf("Persian search/filter result: total=%d items=%+v", searchPage.Total, searchPage.Items)
	}
	for _, tc := range []struct {
		term string
		id   string
	}{
		{term: "سمیرا", id: secondID},
		{term: "store-2", id: secondID},
	} {
		found, err := repo.ListAdmin(ctx, repository.AdminStoreFilter{Search: tc.term, Limit: 5})
		if err != nil || found.Total != 1 || len(found.Items) != 1 || found.Items[0].PublicID != tc.id {
			t.Errorf("search %q: page=%+v err=%v", tc.term, found, err)
		}
	}
	citySearch, err := repo.ListAdmin(ctx, repository.AdminStoreFilter{Search: "تهران", Limit: 5})
	if err != nil || citySearch.Total != 2 {
		t.Fatalf("city search: page=%+v err=%v", citySearch, err)
	}

	falseValue := false
	paused, err := repo.ListAdmin(ctx, repository.AdminStoreFilter{
		AcceptsOrders: &falseValue, Limit: 5,
	})
	if err != nil || paused.Total != 1 || paused.Items[0].PublicID != secondID {
		t.Fatalf("accepts_orders=false filter: page=%+v err=%v", paused, err)
	}

	paged, err := repo.ListAdmin(ctx, repository.AdminStoreFilter{
		Status: string(enum.StoreStatusApproved), Sort: "newest", Limit: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if paged.Total != 2 || len(paged.Items) != 1 || paged.Items[0].PublicID != secondID {
		t.Fatalf("first page: total=%d items=%+v", paged.Total, paged.Items)
	}
	oldest, err := repo.ListAdmin(ctx, repository.AdminStoreFilter{
		Status: string(enum.StoreStatusApproved), Sort: "oldest", Limit: 1,
	})
	if err != nil || len(oldest.Items) != 1 || oldest.Items[0].PublicID != firstID {
		t.Fatalf("oldest sorting: page=%+v err=%v", oldest, err)
	}
	next, err := repo.ListAdmin(ctx, repository.AdminStoreFilter{
		Status: string(enum.StoreStatusApproved), Sort: "newest", Limit: 1,
		Cursor: &repository.AdminStoreCursor{CreatedAt: paged.Items[0].CreatedAt, ID: paged.Items[0].ID},
	})
	if err != nil || len(next.Items) != 1 || next.Items[0].PublicID != firstID {
		t.Fatalf("second page: %+v, %v", next, err)
	}

	excludeDeleted, err := repo.ListAdmin(ctx, repository.AdminStoreFilter{Search: "حذف", Limit: 5})
	if err != nil || excludeDeleted.Total != 0 {
		t.Fatalf("deleted stores should be excluded by default: %+v, %v", excludeDeleted, err)
	}
	includeDeleted, err := repo.ListAdmin(ctx, repository.AdminStoreFilter{Search: "حذف", IncludeDeleted: true, Limit: 5})
	if err != nil || includeDeleted.Total != 1 || len(includeDeleted.Items) != 1 || includeDeleted.Items[0].PublicID != deletedID {
		t.Fatalf("include deleted: %+v, %v", includeDeleted, err)
	}

	store, err := repo.GetByPublicID(ctx, firstID)
	if err != nil {
		t.Fatal(err)
	}
	if store.Location.Latitude != 35.7 || store.Location.Longitude != 51.41 || !store.IsActive {
		t.Fatalf("location or derived IsActive did not round-trip: %+v", store)
	}

	var (
		originalVendorID int64
		originalStatus   string
		originalAdminID  int64
		originalApproved time.Time
		originalRating   float64
		originalCount    int
		originalTrust    float64
		originalMeals    int
		originalCreated  time.Time
		originalPublicID string
		originalOwner    string
		originalSlug     string
		originalReject   *string
	)
	if err := f.pool.QueryRow(ctx, `
SELECT public_id::text, vendor_id, owner_name, slug, status::text, approved_by,
       approved_at, rejection_reason, rating_avg::float8, rating_count,
       trust_score::float8, meals_saved, created_at
FROM stores WHERE public_id = $1::uuid`, firstID).Scan(
		&originalPublicID, &originalVendorID, &originalOwner, &originalSlug, &originalStatus,
		&originalAdminID, &originalApproved, &originalReject, &originalRating,
		&originalCount, &originalTrust, &originalMeals, &originalCreated,
	); err != nil {
		t.Fatal(err)
	}

	store.Name = "نان تازه مدیریت‌شده"
	store.Biography = "شرح تازه"
	store.Location = model.StoreLocation{Latitude: 35.7219, Longitude: 51.3347}
	store.AcceptsOrders = false
	store.WorkingHours = []model.WorkingHour{{DayOfWeek: 0, OpenTime: "08:00", CloseTime: "19:00"}}
	store.Images = []model.StoreImage{{URL: "https://example.test/نان.jpg", IsPrimary: true, SortOrder: 1}}
	store.VendorID = originalVendorID + 100
	store.PublicID = "00000000-0000-4000-8000-000000000001"
	store.VendorStatus = string(enum.StoreStatusRejected)
	store.OwnerName = "مقدار محافظت‌شده"
	store.Slug = "protected-slug"
	store.IsActive = false
	store.RatingAvg = 0
	store.RatingCount = 0
	store.TrustScore = 0
	store.MealsSaved = 0
	store.CreatedAt = time.Time{}
	if err := repo.Update(ctx, store); err != nil {
		t.Fatal(err)
	}

	updated, err := repo.GetByPublicID(ctx, firstID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "نان تازه مدیریت‌شده" || updated.Location.Latitude != 35.7219 ||
		updated.Location.Longitude != 51.3347 || updated.AcceptsOrders ||
		len(updated.WorkingHours) != 1 || len(updated.Images) != 1 ||
		updated.Images[0].URL != "https://example.test/نان.jpg" {
		t.Fatalf("permitted update did not persist: %+v", updated)
	}

	var (
		vendorID int64
		status   string
		adminID  int64
		approved time.Time
		rating   float64
		count    int
		trust    float64
		meals    int
		created  time.Time
		publicID string
		owner    string
		slug     string
		reject   *string
	)
	if err := f.pool.QueryRow(ctx, `
SELECT public_id::text, vendor_id, owner_name, slug, status::text, approved_by,
       approved_at, rejection_reason, rating_avg::float8, rating_count,
       trust_score::float8, meals_saved, created_at
FROM stores WHERE public_id = $1::uuid`, firstID).Scan(
		&publicID, &vendorID, &owner, &slug, &status, &adminID, &approved,
		&reject, &rating, &count, &trust, &meals, &created,
	); err != nil {
		t.Fatal(err)
	}
	if publicID != originalPublicID || vendorID != originalVendorID || owner != originalOwner ||
		slug != originalSlug || status != originalStatus || adminID != originalAdminID ||
		(reject == nil) != (originalReject == nil) ||
		(reject != nil && *reject != *originalReject) ||
		!approved.Equal(originalApproved) || rating != originalRating || count != originalCount ||
		trust != originalTrust || meals != originalMeals || !created.Equal(originalCreated) {
		t.Fatalf("update altered protected or computed columns: vendor=%d status=%s approved_by=%d approved_at=%s rating=%v count=%d trust=%v meals=%d created=%s",
			vendorID, status, adminID, approved, rating, count, trust, meals, created)
	}
}
