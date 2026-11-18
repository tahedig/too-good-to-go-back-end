package service

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"

	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
	"github.com/tahedig/too-good-to-go/backend/pkg/security"
)

const (
	msgAdminStoreNotFound        = "فروشگاه پیدا نشد."
	msgAdminSubscriptionNotFound = "اشتراکی برای این فروشگاه ثبت نشده است."
	msgAdminOfferNotFound        = "محصول پیدا نشد."
	msgAdminStoreInvalidInput    = "اطلاعات واردشده معتبر نیست."
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

func (s *adminStoreService) GetStoreSubscription(ctx context.Context, storeID string) (*model.AdminStoreSubscription, error) {
	if !security.IsID(storeID) {
		return nil, exception.NewNotFound(msgAdminStoreNotFound)
	}
	sub, err := s.repo.GetStoreSubscription(ctx, storeID)
	if err != nil {
		if errors.Is(err, repository.ErrAdminStoreNotFound) {
			return nil, exception.NewNotFound(msgAdminStoreNotFound)
		}
		if errors.Is(err, repository.ErrAdminSubscriptionNotFound) {
			return nil, exception.NewNotFound(msgAdminSubscriptionNotFound)
		}
		return nil, err
	}
	return sub, nil
}

func (s *adminStoreService) ListStoreOffers(ctx context.Context, storeID string) ([]model.AdminOffer, error) {
	if !security.IsID(storeID) {
		return nil, exception.NewNotFound(msgAdminStoreNotFound)
	}
	offers, err := s.repo.ListStoreOffers(ctx, storeID)
	if err != nil {
		if errors.Is(err, repository.ErrAdminStoreNotFound) {
			return nil, exception.NewNotFound(msgAdminStoreNotFound)
		}
		return nil, err
	}
	return offers, nil
}

func (s *adminStoreService) GetOffer(ctx context.Context, offerID string) (*model.AdminOffer, error) {
	if !security.IsID(offerID) {
		return nil, exception.NewNotFound(msgAdminOfferNotFound)
	}
	offer, err := s.repo.GetOffer(ctx, offerID)
	if err != nil {
		if errors.Is(err, repository.ErrAdminOfferNotFound) {
			return nil, exception.NewNotFound(msgAdminOfferNotFound)
		}
		return nil, err
	}
	return offer, nil
}

func (s *adminStoreService) UpdateOfferStatus(ctx context.Context, offerID string, isActive *bool) (*model.AdminOffer, error) {
	if isActive == nil {
		return nil, exception.NewBadRequest(msgAdminStoreInvalidInput, []exception.FieldError{{
			Field:   "is_active",
			Message: "فیلد is_active الزامی است.",
		}})
	}
	if !security.IsID(offerID) {
		return nil, exception.NewNotFound(msgAdminOfferNotFound)
	}
	offer, err := s.repo.UpdateOfferStatus(ctx, offerID, *isActive)
	if err != nil {
		if errors.Is(err, repository.ErrAdminOfferNotFound) {
			return nil, exception.NewNotFound(msgAdminOfferNotFound)
		}
		return nil, err
	}
	if s.log != nil {
		s.log.Info("admin updated offer active status", "offer_id", offerID, "is_active", *isActive)
	}
	return offer, nil
}

var validAdminOrderStatuses = map[string]bool{
	"pending_payment": true,
	"paid":            true,
	"ready":           true,
	"completed":       true,
	"cancelled":       true,
	"expired":         true,
	"no_show":         true,
}

func (s *adminStoreService) ListStoreOrders(ctx context.Context, storeID string, in serviceinterface.AdminStoreOrderListInput) (*repository.AdminStoreOrderPage, error) {
	if !security.IsID(storeID) {
		return nil, exception.NewNotFound(msgAdminStoreNotFound)
	}

	var fields []exception.FieldError
	page, pageSize := 1, DefaultVendorPageSize
	if in.Page != "" {
		n, err := strconv.Atoi(in.Page)
		if err != nil || n < 1 || n > maxVendorPage {
			fields = append(fields, exception.FieldError{Field: "page", Message: "شماره صفحه باید ۱ یا بیشتر باشد."})
		}
		page = n
	}
	if in.PageSize != "" {
		n, err := strconv.Atoi(in.PageSize)
		if err != nil || n < 1 || n > MaxVendorPageSize {
			fields = append(fields, exception.FieldError{Field: "page_size", Message: "اندازه صفحه باید بین ۱ تا ۱۰۰ باشد."})
		}
		pageSize = n
	}

	var statuses []string
	if raw := strings.TrimSpace(in.Status); raw != "" {
		if raw == "active" {
			statuses = []string{"pending_payment", "paid", "ready"}
		} else {
			for _, part := range strings.Split(raw, ",") {
				st := strings.TrimSpace(part)
				if !validAdminOrderStatuses[st] {
					fields = append(fields, exception.FieldError{Field: "status", Message: "وضعیت سفارش معتبر نیست."})
					break
				}
				statuses = append(statuses, st)
			}
		}
	}

	if len(fields) > 0 {
		return nil, exception.NewBadRequest(msgAdminStoreInvalidInput, fields)
	}

	res, err := s.repo.ListStoreOrders(ctx, storeID, repository.AdminStoreOrderQuery{
		Statuses: statuses,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		if errors.Is(err, repository.ErrAdminStoreNotFound) {
			return nil, exception.NewNotFound(msgAdminStoreNotFound)
		}
		return nil, err
	}
	return res, nil
}
