package memory

import (
	"context"
	"sync"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

type userRepository struct {
	mu      sync.Mutex
	byPhone map[string]*model.User
}

func NewUserRepository() repository.UserRepository {
	return &userRepository{byPhone: make(map[string]*model.User)}
}

func (r *userRepository) FindOrCreateByPhone(_ context.Context, newUser *model.User) (*model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if u, ok := r.byPhone[newUser.Phone]; ok {
		cp := *u
		return &cp, nil
	}
	cp := *newUser
	r.byPhone[newUser.Phone] = &cp
	out := cp
	return &out, nil
}
