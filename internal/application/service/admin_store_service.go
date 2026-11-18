package service

import (
	"context"
	"errors"
	"log/slog"

	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
	"github.com/tahedig/too-good-to-go/backend/pkg/security"
)

const (
	msgAdminStoreNotFound     = "فروشگاه پیدا نشد."
	msgAdminStoreInvalidInput = "اطلاعات واردشده معتبر نیست."
)

type adminStoreService struct {
	repo repository.AdminStoreRepository
	log  *slog.Logger
}

func NewAdminStoreService(repo repository.AdminStoreRepository, log *slog.Logger) serviceinterface.AdminStoreService {
	return &adminStoreService{repo: repo, log: log}
}

func (s *adminStoreService) GetStore(ctx context.Context, storeID string) (*model.AdminStoreDetail, error) {
	if !security.IsID(storeID) {
		return nil, exception.NewNotFound(msgAdminStoreNotFound)
	}
	store, err := s.repo.GetStore(ctx, storeID)
	if err != nil {
		if errors.Is(err, repository.ErrAdminStoreNotFound) {
			return nil, exception.NewNotFound(msgAdminStoreNotFound)
		}
		return nil, err
	}
	return store, nil
}

func (s *adminStoreService) UpdateStoreStatus(ctx context.Context, storeID string, isActive *bool) (*model.AdminStoreDetail, error) {
	if isActive == nil {
		return nil, exception.NewBadRequest(msgAdminStoreInvalidInput, []exception.FieldError{{
			Field:   "is_active",
			Message: "فیلد is_active الزامی است.",
		}})
	}
	if !security.IsID(storeID) {
		return nil, exception.NewNotFound(msgAdminStoreNotFound)
	}
	store, err := s.repo.UpdateStoreStatus(ctx, storeID, *isActive)
	if err != nil {
		if errors.Is(err, repository.ErrAdminStoreNotFound) {
			return nil, exception.NewNotFound(msgAdminStoreNotFound)
		}
		return nil, err
	}
	if s.log != nil {
		s.log.Info("admin updated store active status", "store_id", storeID, "is_active", *isActive)
	}
	return store, nil
}
