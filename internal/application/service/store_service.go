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
	address := strings.TrimSpace(updated.PickupAddress)
	instructions := strings.TrimSpace(updated.PickupInstructions)

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

	// Only update fields that vendors are allowed to edit.
	// Ownership, IDs, ratings, trust scores, and statuses remain unchanged.
	store.Name = name
	store.Biography = strings.TrimSpace(updated.Biography)
	store.PickupAddress = address
	store.PickupInstructions = instructions

	if err := s.repo.Update(ctx, store); err != nil {
		return nil, exception.NewInternalServer(err)
	}

	return store, nil
}
