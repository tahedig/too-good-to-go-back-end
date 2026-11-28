package service_test

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/application/service"
	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

const adminManagedStoreID = "11111111-1111-4111-8111-111111111111"

type adminStoreManagementRepositoryStub struct {
	store          *model.Store
	listPage       *repository.AdminStorePage
	listFilter     repository.AdminStoreFilter
	categoryActive bool
	categoryErr    error
	getErr         error
	listErr        error
	updateErr      error
	updated        *model.Store
}

func (r *adminStoreManagementRepositoryStub) ListAdmin(_ context.Context, filter repository.AdminStoreFilter) (*repository.AdminStorePage, error) {
	r.listFilter = filter
	if r.listPage == nil {
		return &repository.AdminStorePage{Items: []*model.Store{}, Total: 0}, r.listErr
	}
	return r.listPage, r.listErr
}

func (r *adminStoreManagementRepositoryStub) GetByPublicID(_ context.Context, _ string) (*model.Store, error) {
	return r.store, r.getErr
}

func (r *adminStoreManagementRepositoryStub) CategoryIsActive(_ context.Context, _ int16) (bool, error) {
	return r.categoryActive, r.categoryErr
}

func (r *adminStoreManagementRepositoryStub) Update(_ context.Context, store *model.Store) error {
	r.updated = store
	return r.updateErr
}

func TestAdminStoreManagementServiceListValidationAndPaging(t *testing.T) {
	store := &model.Store{ID: 10, CreatedAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}
	repo := &adminStoreManagementRepositoryStub{
		listPage: &repository.AdminStorePage{Items: []*model.Store{store, {ID: 9}}, Total: 2},
	}
	svc := service.NewAdminStoreManagementService(repo)
	limit := 1
	categoryID := 3
	acceptsOrders, includeDeleted := false, true

	page, err := svc.ListStores(context.Background(), serviceinterface.AdminStoreListInput{
		Search: " نان ", Status: "approved", CategoryID: &categoryID, City: "تهران",
		AcceptsOrders: &acceptsOrders, IncludeDeleted: &includeDeleted, Sort: "newest", Limit: &limit,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || !page.HasMore || page.NextCursor == nil || page.Total != 2 {
		t.Fatalf("unexpected page: %+v", page)
	}
	if repo.listFilter.Search != "نان" || repo.listFilter.Status != "approved" || repo.listFilter.CategoryID == nil ||
		*repo.listFilter.CategoryID != 3 || repo.listFilter.City != "تهران" || repo.listFilter.AcceptsOrders == nil ||
		*repo.listFilter.AcceptsOrders || !repo.listFilter.IncludeDeleted || repo.listFilter.Limit != 2 {
		t.Fatalf("repository received unexpected filter: %+v", repo.listFilter)
	}

	next, err := svc.ListStores(context.Background(), serviceinterface.AdminStoreListInput{
		Limit: &limit, Cursor: *page.NextCursor,
	})
	if err != nil || len(next.Items) != 1 {
		t.Fatalf("next page: %+v, %v", next, err)
	}

	invalidLimit := 51
	_, err = svc.ListStores(context.Background(), serviceinterface.AdminStoreListInput{Limit: &invalidLimit})
	assertAppErrCode(t, err, exception.CodeBadRequest)
	_, err = svc.ListStores(context.Background(), serviceinterface.AdminStoreListInput{Cursor: "not-a-cursor"})
	assertAppErrCode(t, err, exception.CodeBadRequest)
}

func TestAdminStoreManagementServiceUpdatePermittedFieldsAndProtectsOthers(t *testing.T) {
	phone := "09120000000"
	categoryID := int16(3)
	logo := "https://example.test/logo.png"
	cover := "https://example.test/cover.png"
	district := "ولیعصر"
	original := &model.Store{
		ID: 10, PublicID: adminManagedStoreID, VendorID: 22, VendorStatus: "approved",
		CategoryID: &categoryID, OwnerName: "مالک", Name: "نان فروشی", Slug: "existing-slug",
		Biography: "بیوگرافی", City: "تهران", AddressLine: "نشانی", IsActive: true,
		AcceptsOrders: true, RatingAvg: 4.5, RatingCount: 20, TrustScore: 90, MealsSaved: 30,
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	repo := &adminStoreManagementRepositoryStub{store: original, categoryActive: true}
	svc := service.NewAdminStoreManagementService(repo)
	name, biography, city, address := "  فروشگاه تازه  ", "  معرفی تازه  ", "  تهران  ", "  نشانی تازه  "
	instructions := " راهنمای دریافت "
	location := &model.StoreLocation{Latitude: 35.7, Longitude: 51.4}
	acceptsOrders := false
	hours := []model.WorkingHour{{DayOfWeek: 0, OpenTime: "08:00", CloseTime: "19:00"}}
	images := []model.StoreImage{{URL: "https://example.test/store.png", IsPrimary: true, SortOrder: 1}}

	got, err := svc.UpdateManagedStore(context.Background(), adminManagedStoreID, &model.AdminStoreUpdate{
		Name: &name, Biography: &biography, PickupInstructions: &instructions, Phone: &phone,
		City: &city, District: &district, AddressLine: &address, CategoryID: &categoryID,
		Location: location, LogoURL: &logo, CoverImageURL: &cover, WorkingHours: &hours,
		Images: &images, AcceptsOrders: &acceptsOrders,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "فروشگاه تازه" || got.Biography != "معرفی تازه" || got.PickupInstructions != "راهنمای دریافت" ||
		got.Phone == nil || *got.Phone != phone || got.City != "تهران" || got.AddressLine != "نشانی تازه" ||
		got.Location != *location || got.CategoryID == nil || *got.CategoryID != categoryID ||
		got.LogoURL == nil || *got.LogoURL != logo || got.CoverImageURL == nil || *got.CoverImageURL != cover ||
		got.AcceptsOrders || len(got.WorkingHours) != 1 || len(got.Images) != 1 {
		t.Fatalf("permitted update fields not applied: %+v", got)
	}
	if got.VendorID != 22 || got.PublicID != adminManagedStoreID || got.VendorStatus != "approved" ||
		got.OwnerName != "مالک" || got.Slug != "existing-slug" || !got.IsActive ||
		got.RatingAvg != 4.5 || got.RatingCount != 20 || got.TrustScore != 90 || got.MealsSaved != 30 ||
		!got.CreatedAt.Equal(original.CreatedAt) {
		t.Fatalf("protected or computed values changed: %+v", got)
	}
	if repo.updated != original {
		t.Fatal("repository Update did not receive the loaded store")
	}
}

func TestAdminStoreManagementServiceRejectsInvalidUpdates(t *testing.T) {
	badLocations := []*model.StoreLocation{
		{Latitude: 91, Longitude: 0},
		{Latitude: 0, Longitude: 181},
		{Latitude: math.NaN(), Longitude: 0},
		{Latitude: 0, Longitude: math.Inf(1)},
	}
	for _, location := range badLocations {
		repo := &adminStoreManagementRepositoryStub{store: &model.Store{
			ID: 1, PublicID: adminManagedStoreID, Name: "Valid", Biography: "Valid",
			City: "Tehran", AddressLine: "Address",
		}, categoryActive: true}
		svc := service.NewAdminStoreManagementService(repo)
		_, err := svc.UpdateManagedStore(context.Background(), adminManagedStoreID, &model.AdminStoreUpdate{
			Location: location,
		})
		assertAppErrCode(t, err, exception.CodeBadRequest)
		if repo.updated != nil {
			t.Fatal("invalid coordinates reached repository update")
		}
	}

	repo := &adminStoreManagementRepositoryStub{store: &model.Store{
		ID: 1, PublicID: adminManagedStoreID, Name: "Valid", Biography: "Valid",
		City: "Tehran", AddressLine: "Address",
	}, categoryActive: false}
	svc := service.NewAdminStoreManagementService(repo)
	category := int16(3)
	_, err := svc.UpdateManagedStore(context.Background(), adminManagedStoreID, &model.AdminStoreUpdate{CategoryID: &category})
	assertAppErrCode(t, err, exception.CodeBadRequest)
	if repo.updated != nil {
		t.Fatal("inactive category reached repository update")
	}

	for _, field := range []string{"name", "biography", "city", "address_line"} {
		t.Run("blank "+field, func(t *testing.T) {
			repo := &adminStoreManagementRepositoryStub{store: &model.Store{
				ID: 1, PublicID: adminManagedStoreID, Name: "Valid", Biography: "Valid",
				City: "Tehran", AddressLine: "Address",
			}, categoryActive: true}
			svc := service.NewAdminStoreManagementService(repo)
			blank := "  "
			update := &model.AdminStoreUpdate{}
			switch field {
			case "name":
				update.Name = &blank
			case "biography":
				update.Biography = &blank
			case "city":
				update.City = &blank
			case "address_line":
				update.AddressLine = &blank
			}
			_, err := svc.UpdateManagedStore(context.Background(), adminManagedStoreID, update)
			assertAppErrCode(t, err, exception.CodeBadRequest)
			if repo.updated != nil {
				t.Fatalf("blank %s reached repository update", field)
			}
		})
	}

	_, err = svc.GetManagedStore(context.Background(), "not-a-uuid")
	assertAppErrCode(t, err, exception.CodeBadRequest)

	repo = &adminStoreManagementRepositoryStub{getErr: repository.ErrStoreNotFound}
	svc = service.NewAdminStoreManagementService(repo)
	_, err = svc.GetManagedStore(context.Background(), adminManagedStoreID)
	assertAppErrCode(t, err, exception.CodeNotFound)
}

func TestAdminStoreManagementServicePropagatesRepositoryFailures(t *testing.T) {
	repo := &adminStoreManagementRepositoryStub{store: &model.Store{
		ID: 1, PublicID: adminManagedStoreID, Name: "Valid", Biography: "Valid",
		City: "Tehran", AddressLine: "Address",
	}, categoryActive: true, updateErr: errors.New("database unavailable")}
	svc := service.NewAdminStoreManagementService(repo)
	name := "Updated"
	_, err := svc.UpdateManagedStore(context.Background(), adminManagedStoreID, &model.AdminStoreUpdate{Name: &name})
	assertAppErrCode(t, err, exception.CodeInternalServer)
}
