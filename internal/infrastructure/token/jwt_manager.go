// Package token implements serviceinterface.TokenManager with HS256 JWTs.
package token

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
)

type claims struct {
	Role enum.Role `json:"role"`
	jwt.RegisteredClaims
}

type jwtManager struct {
	secret []byte
	issuer string
	ttl    time.Duration
	now    func() time.Time
}

func NewJWTManager(secret, issuer string, ttl time.Duration) serviceinterface.TokenManager {
	return &jwtManager{secret: []byte(secret), issuer: issuer, ttl: ttl, now: time.Now}
}

func (m *jwtManager) Issue(user *model.User) (string, time.Duration, error) {
	now := m.now()
	c := claims{
		Role: user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID,
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(m.secret)
	if err != nil {
		return "", 0, fmt.Errorf("sign token: %w", err)
	}
	return signed, m.ttl, nil
}

func (m *jwtManager) Parse(raw string) (*serviceinterface.TokenClaims, error) {
	var c claims
	_, err := jwt.ParseWithClaims(raw, &c,
		func(*jwt.Token) (any, error) { return m.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(m.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(m.now),
	)
	if err != nil || c.Subject == "" || !c.Role.Valid() {
		return nil, serviceinterface.ErrInvalidToken
	}
	return &serviceinterface.TokenClaims{UserID: c.Subject, Role: c.Role}, nil
}
