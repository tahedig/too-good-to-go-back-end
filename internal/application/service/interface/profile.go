package serviceinterface

import (
	"context"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
)

// ProfileService reads and updates the authenticated user's editable profile.
type ProfileService interface {
	Get(ctx context.Context, userID string) (*model.User, error)
	UpdateFullName(ctx context.Context, userID, fullName string) (*model.User, error)
}
