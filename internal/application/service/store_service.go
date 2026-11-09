package service

import (
	"context"
	"strings"

	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

type storeService struct {
	repo repository.StoreRepository
}

// NewStoreService creates a StoreService.
func NewStoreService(repo repository.StoreRepository) serviceinterface.StoreService {
	return &storeService{repo: repo}
}

func (s *storeService) ListStores(ctx context.Context, userID int64) ([]*model.Store, error) {
	stores, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, exception.NewInternalServer(err)
	}

	return stores, nil
}

func (s *storeService) GetStore(ctx context.Context, userID int64, publicID string) (*model.Store, error) {
	store, err := s.repo.GetByPublicIDAndUser(ctx, publicID, userID)
	if err != nil {
		return nil, exception.NewInternalServer(err)
	}

	if store == nil {
		return nil, exception.NewNotFound("store not found")
	}

	return store, nil
}

func (s *storeService) UpdateStore(
	ctx context.Context,
	userID int64,
	publicID string,
	updated *model.Store,
) (*model.Store, error) {
	if updated == nil {
		return nil, exception.NewBadRequest("store update data is required", nil)
	}

	name := strings.TrimSpace(updated.Name)
	address := strings.TrimSpace(updated.AddressLine)

	var fieldErrors []exception.FieldError

	if name == "" {
		fieldErrors = append(fieldErrors, exception.FieldError{
			Field:   "name",
			Message: "store name is required",
		})
	}

	if address == "" {
		fieldErrors = append(fieldErrors, exception.FieldError{
			Field:   "pickup_address",
			Message: "pickup address is required",
		})
	}

	if len(fieldErrors) > 0 {
		return nil, exception.NewBadRequest("invalid store information", fieldErrors)
	}

	store, err := s.repo.GetByPublicIDAndUser(ctx, publicID, userID)
	if err != nil {
		return nil, exception.NewInternalServer(err)
	}

	if store == nil {
		return nil, exception.NewNotFound("store not found")
	}

	// Update all editable store form fields
	store.Name = name
	store.AddressLine = address
	store.Biography = strings.TrimSpace(updated.Biography)
	store.PickupInstructions = strings.TrimSpace(updated.PickupInstructions)

	if updated.CategoryID != nil {
		store.CategoryID = updated.CategoryID
	}

	if updated.Phone != nil {
		trimmedPhone := strings.TrimSpace(*updated.Phone)
		store.Phone = &trimmedPhone
	}

	if updated.Location.Latitude != 0 || updated.Location.Longitude != 0 {
		store.Location = updated.Location
	}

	if len(updated.WorkingHours) > 0 {
		store.WorkingHours = updated.WorkingHours
	}

	if len(updated.Images) > 0 {
		store.Images = updated.Images
	}

	// Ownership, VendorID, VendorStatus, RatingAvg, TrustScore, IsActive,
	// and AcceptsOrders remain strictly protected.

	if err := s.repo.Update(ctx, store); err != nil {
		return nil, exception.NewInternalServer(err)
	}

	return store, nil
}
