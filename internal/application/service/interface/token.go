package serviceinterface

import (
	"errors"
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
)

// ErrInvalidToken is returned by TokenManager.Parse for any token that is
// malformed, badly signed, expired or has the wrong issuer.
var ErrInvalidToken = errors.New("invalid token")

// TokenClaims is what the API trusts from a verified access token.
type TokenClaims struct {
	UserID string
	Role   enum.Role
}

// TokenManager issues and verifies access tokens. The JWT implementation lives
// in infrastructure/token.
type TokenManager interface {
	Issue(user *model.User) (token string, expiresIn time.Duration, err error)
	Parse(token string) (*TokenClaims, error)
}
