package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
	"github.com/tahedig/too-good-to-go/backend/pkg/security"
)

const (
	DefaultVendorDashboardRecentLimit = 10
	MaxVendorDashboardRecentLimit     = 50
)

type vendorDashboardService struct {
	repo repository.VendorDashboardRepository
	now  func() time.Time
}

// NewVendorDashboardService constructs a VendorDashboardService backed by repo.
func NewVendorDashboardService(repo repository.VendorDashboardRepository) serviceinterface.VendorDashboardService {
	return &vendorDashboardService{repo: repo, now: time.Now}
}

func (s *vendorDashboardService) GetDashboard(
	ctx context.Context,
	vendorUserID string,
	in serviceinterface.VendorDashboardInput,
) (*model.VendorDashboard, error) {
	if vendorUserID == "" {
		return nil, exception.NewUnauthorized("authentication required")
	}

	q, err := parseVendorDashboardInput(vendorUserID, in, s.now().UTC())
	if err != nil {
		return nil, err
	}

	dash, err := s.repo.GetDashboard(ctx, q)
	if errors.Is(err, repository.ErrStoreNotFound) {
		return nil, exception.NewNotFound(msgVendorNotFound)
	}
	if err != nil {
		return nil, exception.NewInternalServer(err)
	}
	return dash, nil
}

func parseVendorDashboardInput(vendorUserID string, in serviceinterface.VendorDashboardInput, now time.Time) (repository.VendorDashboardQuery, error) {
	var (
		q = repository.VendorDashboardQuery{
			VendorUserID: vendorUserID,
			RecentLimit:  DefaultVendorDashboardRecentLimit,
			Now:          now,
		}
		fields []exception.FieldError
	)

	if storeID := strings.TrimSpace(in.StoreID); storeID != "" {
		if !security.IsID(storeID) {
			fields = append(fields, exception.FieldError{
				Field:   "store_id",
				Message: "must be a valid UUID",
			})
		} else {
			q.StoreID = strings.ToLower(storeID)
		}
	}

	if raw := strings.TrimSpace(in.RecentLimit); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > MaxVendorDashboardRecentLimit {
			fields = append(fields, exception.FieldError{
				Field:   "recent_limit",
				Message: "must be a number from 1 to " + strconv.Itoa(MaxVendorDashboardRecentLimit),
			})
		} else {
			q.RecentLimit = n
		}
	}

	if len(fields) > 0 {
		return q, exception.NewBadRequest("invalid vendor dashboard query", fields)
	}
	return q, nil
}
