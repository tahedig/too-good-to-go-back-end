package repository

import (
	"context"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
)

type UserRepository interface {
	// FindOrCreateByPhone returns the user with this phone, creating it with
	// newUser's values if it does not exist yet. It must be atomic so two
	// parallel first logins cannot create two users.
	FindOrCreateByPhone(ctx context.Context, newUser *model.User) (*model.User, error)
}
