package service

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

const maxProfileNameLength = 100

type profileService struct {
	users repository.UserRepository
}

// NewProfileService creates a ProfileService backed by the account repository.
func NewProfileService(users repository.UserRepository) serviceinterface.ProfileService {
	return &profileService{users: users}
}

func (s *profileService) Get(ctx context.Context, userID string) (*model.User, error) {
	user, err := s.users.FindByID(ctx, userID)
	if errors.Is(err, repository.ErrUserNotFound) {
		return nil, exception.NewNotFound("user not found")
	}
	if err != nil {
		return nil, exception.NewInternalServer(err)
	}
	return user, nil
}

func (s *profileService) UpdateFullName(
	ctx context.Context,
	userID string,
	fullName string,
) (*model.User, error) {
	fullName = strings.TrimSpace(fullName)
	if utf8.RuneCountInString(fullName) == 0 || utf8.RuneCountInString(fullName) > maxProfileNameLength {
		return nil, exception.NewBadRequest("invalid display name", []exception.FieldError{{
			Field:   "display_name",
			Message: "must contain between 1 and 100 characters",
		}})
	}

	if err := s.users.UpdateFullName(ctx, userID, fullName); errors.Is(err, repository.ErrUserNotFound) {
		return nil, exception.NewNotFound("user not found")
	} else if err != nil {
		return nil, exception.NewInternalServer(err)
	}
	return s.Get(ctx, userID)
}
