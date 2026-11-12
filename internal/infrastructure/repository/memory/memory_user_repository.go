package memory

import (
	"context"
	"sync"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

// UserRepository keeps users in memory. It is used when DATABASE_URL is not
// set (local development) and in tests.
type UserRepository struct {
	mu      sync.Mutex
	byPhone map[string]*model.User
	byID    map[string]*model.User
}

func NewUserRepository() *UserRepository {
	return &UserRepository{byPhone: make(map[string]*model.User), byID: make(map[string]*model.User)}
}

var _ repository.UserRepository = (*UserRepository)(nil)

func (r *UserRepository) FindOrCreateByPhone(_ context.Context, newUser *model.User) (*model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if u, ok := r.byPhone[newUser.Phone]; ok {
		cp := *u
		return &cp, nil
	}
	stored := *newUser
	r.byPhone[stored.Phone] = &stored
	r.byID[stored.ID] = &stored
	out := stored
	return &out, nil
}

func (r *UserRepository) FindByID(_ context.Context, id string) (*model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.byID[id]
	if !ok {
		return nil, repository.ErrUserNotFound
	}
	cp := *u
	return &cp, nil
}

// SetAccess replaces a stored user's role and status, like an admin action
// on the database would. It reports false when the user does not exist.
func (r *UserRepository) SetAccess(id string, role enum.Role, status model.UserStatus) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.byID[id]
	if !ok {
		return false
	}
	u.Role, u.Status = role, status
	return true
}
