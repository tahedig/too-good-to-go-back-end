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

// SessionService issues and checks sessions.
type SessionService interface {
	// Login is called for a user whose phone number has already been
	// verified (e.g. by OTP): it loads or creates the user and issues an
	// access token. It never checks credentials itself.
	Login(ctx context.Context, verifiedPhone string) (*AuthResult, error)
	// Authenticate verifies an access token and reloads its user, so a
	// suspended or deleted account, or one whose roles were revoked, is
	// refused even while its token has not expired. The returned user carries
	// the current role from storage, not the one in the token.
	Authenticate(ctx context.Context, accessToken string) (*model.User, error)
}
