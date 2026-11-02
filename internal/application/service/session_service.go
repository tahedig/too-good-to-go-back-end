package service

import (
	"context"
	"log/slog"
	"time"

	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
	"github.com/tahedig/too-good-to-go/backend/pkg/logger"
	"github.com/tahedig/too-good-to-go/backend/pkg/security"
	"github.com/tahedig/too-good-to-go/backend/pkg/validation"
)

type sessionService struct {
	users  repository.UserRepository
	tokens serviceinterface.TokenManager
	roles  map[string]enum.Role
	log    *slog.Logger
	now    func() time.Time
}

// NewSessionService returns the login half of authentication. phoneRoles
// maps normalized phone numbers to the role a brand-new user with that
// number receives; everyone else becomes a customer. Roles of existing users
// are never changed here.
func NewSessionService(
	users repository.UserRepository,
	tokens serviceinterface.TokenManager,
	phoneRoles map[string]enum.Role,
	log *slog.Logger,
) serviceinterface.SessionService {
	return &sessionService{users: users, tokens: tokens, roles: phoneRoles, log: log, now: time.Now}
}

func (s *sessionService) Login(ctx context.Context, verifiedPhone string) (*serviceinterface.AuthResult, error) {
	phone, err := validation.NormalizePhone(verifiedPhone)
	if err != nil {
		return nil, exception.NewBadRequest("phone number is not valid",
			[]exception.FieldError{{Field: "phone", Message: "must be a valid phone number"}})
	}

	user, err := s.findOrCreateUser(ctx, phone)
	if err != nil {
		return nil, exception.NewInternalServer(err)
	}
	accessToken, expiresIn, err := s.tokens.Issue(user)
	if err != nil {
		return nil, exception.NewInternalServer(err)
	}

	logger.FromContext(ctx, s.log).Info("session issued",
		"phone", validation.MaskPhone(phone), "user_id", user.ID, "role", string(user.Role))
	return &serviceinterface.AuthResult{User: user, AccessToken: accessToken, ExpiresIn: expiresIn}, nil
}

func (s *sessionService) findOrCreateUser(ctx context.Context, phone string) (*model.User, error) {
	role, ok := s.roles[phone]
	if !ok {
		role = enum.RoleCustomer
	}
	id, err := security.NewID()
	if err != nil {
		return nil, err
	}
	return s.users.FindOrCreateByPhone(ctx, &model.User{
		ID:        id,
		Phone:     phone,
		Role:      role,
		CreatedAt: s.now().UTC(),
	})
}
