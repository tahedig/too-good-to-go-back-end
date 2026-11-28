package service

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"time"

	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

const (
	DefaultAdminDashboardActivityLimit = 10
	MaxAdminDashboardActivityLimit     = 50
)

type adminDashboardService struct {
	repo repository.AdminDashboardRepository
	now  func() time.Time
}

// NewAdminDashboardService constructs an AdminDashboardService backed by repo.
func NewAdminDashboardService(repo repository.AdminDashboardRepository) serviceinterface.AdminDashboardService {
	return &adminDashboardService{repo: repo, now: time.Now}
}

func (s *adminDashboardService) GetDashboard(
	ctx context.Context,
	in serviceinterface.AdminDashboardInput,
) (*model.AdminDashboard, error) {
	q, err := parseAdminDashboardInput(in, s.now().UTC())
	if err != nil {
		return nil, err
	}

	dash, err := s.repo.GetDashboard(ctx, q)
	if err != nil {
		return nil, exception.NewInternalServer(err)
	}
	return dash, nil
}

func parseAdminDashboardInput(in serviceinterface.AdminDashboardInput, now time.Time) (repository.AdminDashboardQuery, error) {
	var (
		q = repository.AdminDashboardQuery{
			ActivityLimit: DefaultAdminDashboardActivityLimit,
			Now:           now,
		}
		fields []exception.FieldError
	)

	if raw := strings.TrimSpace(in.ActivityLimit); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > MaxAdminDashboardActivityLimit {
			fields = append(fields, exception.FieldError{
				Field:   "activity_limit",
				Message: "must be a number from 1 to " + strconv.Itoa(MaxAdminDashboardActivityLimit),
			})
		} else {
			q.ActivityLimit = n
		}
	}

	if rawTypes := strings.TrimSpace(in.ActivityType); rawTypes != "" {
		var wanted []model.AdminActivityType
		for _, part := range strings.Split(rawTypes, ",") {
			at := model.AdminActivityType(strings.TrimSpace(part))
			if !at.Valid() {
				fields = append(fields, exception.FieldError{
					Field:   "activity_type",
					Message: "must be a comma-separated list of: store_application, store_decision, reservation",
				})
				break
			}
			if !slices.Contains(wanted, at) {
				wanted = append(wanted, at)
			}
		}
		q.ActivityTypes = wanted
	}

	if len(fields) > 0 {
		return q, exception.NewBadRequest("invalid admin dashboard query", fields)
	}
	return q, nil
}
