package repository

import (
	"context"
	"errors"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
)

// ErrUserNotFound is returned by FindByID when no user has the given ID.
var ErrUserNotFound = errors.New("user not found")

type UserRepository interface {
	// FindOrCreateByPhone returns the user with this phone, creating it with
	// newUser's values if it does not exist yet. It must be atomic so two
	// parallel first logins cannot create two users.
	FindOrCreateByPhone(ctx context.Context, newUser *model.User) (*model.User, error)
	// FindByID returns the current state of the user with this public ID, or
	// ErrUserNotFound. Role is empty when the user holds no application role.
	FindByID(ctx context.Context, id string) (*model.User, error)
}
