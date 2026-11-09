package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

type storeRequestService struct {
	repo repository.StoreRequestRepository
}

// NewStoreRequestService creates a StoreRequestService.
func NewStoreRequestService(repo repository.StoreRequestRepository) serviceinterface.StoreRequestService {
	return &storeRequestService{repo: repo}
}

func (s *storeRequestService) ListStoreRequests(ctx context.Context, userID int64) ([]*model.StoreRequest, error) {
	requests, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, exception.NewInternalServer(err)
	}

	return requests, nil
}

func (s *storeRequestService) GetStoreRequest(ctx context.Context, userID int64, publicID string) (*model.StoreRequest, error) {
	req, err := s.repo.GetByPublicIDAndUser(ctx, publicID, userID)
	if err != nil {
		return nil, exception.NewInternalServer(err)
	}

	if req == nil {
		return nil, exception.NewNotFound("store request not found")
	}

	return req, nil
}

func (s *storeRequestService) CreateStoreRequest(
	ctx context.Context,
	userID int64,
	req *model.StoreRequest,
) (*model.StoreRequest, error) {
	if req == nil {
		return nil, exception.NewBadRequest("store request data is required", nil)
	}

	storeName := strings.TrimSpace(req.StoreName)
	address := strings.TrimSpace(req.Address)

	var fieldErrors []exception.FieldError
	if storeName == "" {
		fieldErrors = append(fieldErrors, exception.FieldError{
			Field:   "store_name",
			Message: "store name is required",
		})
	}
	if address == "" {
		fieldErrors = append(fieldErrors, exception.FieldError{
			Field:   "address",
			Message: "address is required",
		})
	}

	if len(fieldErrors) > 0 {
		return nil, exception.NewBadRequest("invalid store request information", fieldErrors)
	}

	now := time.Now().UTC()
	req.VendorID = userID
	req.StoreName = storeName
	req.Address = address
	req.OwnerName = strings.TrimSpace(req.OwnerName)
	req.Status = "pending_review"
	req.StatusFa = "در انتظار بررسی"
	req.CreatedAt = now
	req.UpdatedAt = now

	if req.PublicID == "" {
		req.PublicID = generateRequestPublicID()
	}

	if err := s.repo.Create(ctx, req); err != nil {
		return nil, exception.NewInternalServer(err)
	}

	return req, nil
}

func generateRequestPublicID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("req-%x", b)
}
