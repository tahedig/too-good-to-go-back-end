package serviceinterface

import (
	"context"
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
)

// AuthResult is a freshly issued session.
type AuthResult struct {
	User        *model.User
	AccessToken string
	ExpiresIn   time.Duration
}

// SessionService logs in a user whose phone number has already been
// verified (e.g. by OTP): it loads or creates the user and issues an access
// token. It never checks credentials itself.
type SessionService interface {
	Login(ctx context.Context, verifiedPhone string) (*AuthResult, error)
}
