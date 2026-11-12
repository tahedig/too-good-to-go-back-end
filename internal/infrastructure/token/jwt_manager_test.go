package token

import (
	"errors"
	"strings"
	"testing"
	"time"

	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
)

const secret = "test-secret-test-secret-test-secret-123"

func TestIssueAndParse(t *testing.T) {
	m := NewJWTManager(secret, "issuer", time.Hour)
	tok, ttl, err := m.Issue(&model.User{ID: "user-1", Role: enum.RoleVendor})
	if err != nil || ttl != time.Hour || strings.Count(tok, ".") != 2 {
		t.Fatalf("Issue: token=%q ttl=%v err=%v", tok, ttl, err)
	}

	claims, err := m.Parse(tok)
	if err != nil || claims.UserID != "user-1" || claims.Role != enum.RoleVendor {
		t.Fatalf("Parse: %+v err=%v", claims, err)
	}
}

func TestParseRejectsBadTokens(t *testing.T) {
	m := NewJWTManager(secret, "issuer", time.Hour).(*jwtManager)
	valid, _, _ := m.Issue(&model.User{ID: "user-1", Role: enum.RoleAdmin})

	otherSecret, _, _ := NewJWTManager("another-secret-another-secret-12345", "issuer", time.Hour).Issue(&model.User{ID: "user-1", Role: enum.RoleAdmin})
	otherIssuer, _, _ := NewJWTManager(secret, "someone-else", time.Hour).Issue(&model.User{ID: "user-1", Role: enum.RoleAdmin})
	noSubject, _, _ := m.Issue(&model.User{Role: enum.RoleAdmin})
	badRole, _, _ := m.Issue(&model.User{ID: "user-1", Role: enum.Role("root")})

	for name, tok := range map[string]string{
		"garbage":      "not.a.jwt",
		"tampered":     valid[:len(valid)-2] + "xx",
		"wrong secret": otherSecret,
		"wrong issuer": otherIssuer,
		"no subject":   noSubject,
		"unknown role": badRole,
	} {
		if _, err := m.Parse(tok); !errors.Is(err, serviceinterface.ErrInvalidToken) {
			t.Errorf("%s: err=%v, want ErrInvalidToken", name, err)
		}
	}

	// Expiry is checked against the manager's clock.
	m.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	if _, err := m.Parse(valid); !errors.Is(err, serviceinterface.ErrInvalidToken) {
		t.Fatalf("expired token accepted: err=%v", err)
	}
}
