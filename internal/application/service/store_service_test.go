package service

import (
	"context"
	"errors"
	"testing"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

type storeRepositoryStub struct {
	stores    []*model.Store
	store     *model.Store
	err       error
	updateErr error
	updated   *model.Store
	userID    int64
	publicID  string
}

var _ repository.StoreRepository = (*storeRepositoryStub)(nil)

func (r *storeRepositoryStub) ListByUser(
	_ context.Context,
	userID int64,
) ([]*model.Store, error) {
	r.userID = userID
	return r.stores, r.err
}

func (r *storeRepositoryStub) GetByPublicIDAndUser(
	_ context.Context,
	publicID string,
	userID int64,
) (*model.Store, error) {
	r.publicID = publicID
	r.userID = userID
	return r.store, r.err
}

func (r *storeRepositoryStub) Update(
	_ context.Context,
	store *model.Store,
) error {
	r.updated = store
	return r.updateErr
}

func TestStoreServiceListStores(t *testing.T) {
	expected := []*model.Store{
		{PublicID: "store-1", Name: "Store One"},
	}

	repo := &storeRepositoryStub{stores: expected}
	svc := NewStoreService(repo)

	stores, err := svc.ListStores(context.Background(), 42)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(stores) != 1 || stores[0].PublicID != "store-1" {
		t.Fatalf("unexpected stores: %+v", stores)
	}

	if repo.userID != 42 {
		t.Fatalf("expected user ID 42, got %d", repo.userID)
	}
}

func TestStoreServiceGetStore(t *testing.T) {
	expected := &model.Store{
		PublicID: "store-1",
		Name:     "Store One",
	}

	repo := &storeRepositoryStub{store: expected}
	svc := NewStoreService(repo)

	store, err := svc.GetStore(context.Background(), 42, "store-1")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if store != expected {
		t.Fatal("expected the retrieved store")
	}

	if repo.userID != 42 || repo.publicID != "store-1" {
		t.Fatalf("unexpected repository arguments: userID=%d publicID=%s",
			repo.userID, repo.publicID)
	}
}

func TestStoreServiceGetStoreNotFound(t *testing.T) {
	repo := &storeRepositoryStub{}
	svc := NewStoreService(repo)

	store, err := svc.GetStore(context.Background(), 42, "missing-store")

	if err == nil {
		t.Fatal("expected not found error")
	}

	if store != nil {
		t.Fatalf("expected nil store, got %+v", store)
	}
}

func TestStoreServiceUpdateStore(t *testing.T) {
	existing := &model.Store{
		PublicID:            "store-1",
		VendorID:            7,
		VendorStatus:        "approved",
		Name:                "Old Name",
		Biography:           "Old biography",
		PickupInstructions:  "Old instructions",
		AddressLine:         "Old address",
		RatingAvg:           4.5,
		TrustScore:          90,
		IsActive:            true,
		AcceptsOrders:       true,
	}

	repo := &storeRepositoryStub{store: existing}
	svc := NewStoreService(repo)

	phone := "09124550120"
	catID := int16(1)

	updated, err := svc.UpdateStore(
		context.Background(),
		42,
		"store-1",
		&model.Store{
			Name:               "  New Name  ",
			AddressLine:        "  New address  ",
			Biography:          "  New biography  ",
			PickupInstructions: "  New instructions  ",
			Phone:              &phone,
			CategoryID:         &catID,
			Location:           model.StoreLocation{Latitude: 35.7, Longitude: 51.4},
			WorkingHours: []model.WorkingHour{
				{DayOfWeek: 0, DayNameFa: "شنبه", OpenTime: "08:00", CloseTime: "19:00", IsClosed: false},
				{DayOfWeek: 6, DayNameFa: "جمعه", IsClosed: true},
			},
			Images: []model.StoreImage{
				{ID: "img-1", URL: "https://example.com/1.jpg", IsPrimary: true, SortOrder: 1},
			},
			VendorID:      999,
			VendorStatus:  "rejected",
			RatingAvg:     1,
			TrustScore:    1,
			IsActive:      false,
			AcceptsOrders: false,
		},
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all editable fields were updated
	if updated.Name != "New Name" ||
		updated.AddressLine != "New address" ||
		updated.Biography != "New biography" ||
		updated.PickupInstructions != "New instructions" ||
		*updated.Phone != "09124550120" ||
		*updated.CategoryID != 1 ||
		updated.Location.Latitude != 35.7 ||
		len(updated.WorkingHours) != 2 ||
		len(updated.Images) != 1 {
		t.Fatalf("editable fields were not updated correctly: %+v", updated)
	}

	// Verify protected fields were NOT modified
	if updated.VendorID != 7 ||
		updated.VendorStatus != "approved" ||
		updated.RatingAvg != 4.5 ||
		updated.TrustScore != 90 ||
		!updated.IsActive ||
		!updated.AcceptsOrders {
		t.Fatalf("protected fields were modified: %+v", updated)
	}

	if repo.updated != existing {
		t.Fatal("expected repository Update to receive the existing store")
	}
}

func TestStoreServiceUpdateStoreValidation(t *testing.T) {
	tests := []struct {
		name    string
		updated *model.Store
	}{
		{
			name:    "nil request",
			updated: nil,
		},
		{
			name: "missing name",
			updated: &model.Store{
				AddressLine: "Some address",
			},
		},
		{
			name: "missing pickup address",
			updated: &model.Store{
				Name: "Some store",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &storeRepositoryStub{}
			svc := NewStoreService(repo)

			_, err := svc.UpdateStore(
				context.Background(),
				42,
				"store-1",
				tt.updated,
			)

			if err == nil {
				t.Fatal("expected validation error")
			}

			if repo.publicID != "" {
				t.Fatal("repository should not be called for invalid input")
			}
		})
	}
}

func TestStoreServiceRepositoryErrors(t *testing.T) {
	repositoryErr := errors.New("database unavailable")

	t.Run("list", func(t *testing.T) {
		repo := &storeRepositoryStub{err: repositoryErr}
		svc := NewStoreService(repo)

		_, err := svc.ListStores(context.Background(), 42)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("get", func(t *testing.T) {
		repo := &storeRepositoryStub{err: repositoryErr}
		svc := NewStoreService(repo)

		_, err := svc.GetStore(context.Background(), 42, "store-1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("update lookup", func(t *testing.T) {
		repo := &storeRepositoryStub{err: repositoryErr}
		svc := NewStoreService(repo)

		_, err := svc.UpdateStore(
			context.Background(),
			42,
			"store-1",
			&model.Store{Name: "Store", AddressLine: "Address"},
		)

		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("update persistence", func(t *testing.T) {
		repo := &storeRepositoryStub{store: &model.Store{}}
		repo.updateErr = repositoryErr
		svc := NewStoreService(repo)

		_, err := svc.UpdateStore(
			context.Background(),
			42,
			"store-1",
			&model.Store{Name: "Store", AddressLine: "Address"},
		)

		if err == nil {
			t.Fatal("expected error")
		}
	})
}
