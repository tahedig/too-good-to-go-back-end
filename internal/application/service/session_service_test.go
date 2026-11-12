package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
	"github.com/tahedig/too-good-to-go/backend/internal/infrastructure/repository/memory"
	"github.com/tahedig/too-good-to-go/backend/internal/infrastructure/token"
)

const (
	customerPhone = "+989123456789"
	vendorPhone   = "+989121111111"
	adminPhone    = "+989122222222"
)

func newSessionService() (*sessionService, func(string) (string, enum.Role)) {
	tokens := token.NewJWTManager("test-secret-test-secret-test-secret-123", "test", time.Hour)
	svc := NewSessionService(memory.NewUserRepository(), tokens,
		map[string]enum.Role{vendorPhone: enum.RoleVendor, adminPhone: enum.RoleAdmin}, discard).(*sessionService)
	parse := func(tok string) (string, enum.Role) {
		c, err := tokens.Parse(tok)
		if err != nil {
			return "", ""
		}
		return c.UserID, c.Role
	}
	return svc, parse
}

func TestLoginCreatesThenReusesUser(t *testing.T) {
	svc, parse := newSessionService()
	ctx := context.Background()

	first, err := svc.Login(ctx, "09123456789") // local format is normalized
	if err != nil {
		t.Fatal(err)
	}
	if first.User.ID == "" || first.User.Phone != customerPhone || first.User.Role != enum.RoleCustomer {
		t.Fatalf("new users should be customers: %+v", first.User)
	}
	if first.AccessToken == "" || first.ExpiresIn != time.Hour {
		t.Fatalf("unexpected token: %+v", first)
	}
	if id, role := parse(first.AccessToken); id != first.User.ID || role != enum.RoleCustomer {
		t.Fatalf("token claims mismatch: id=%q role=%q", id, role)
	}

	second, err := svc.Login(ctx, customerPhone)
	if err != nil || second.User.ID != first.User.ID {
		t.Fatalf("same phone must map to the same user: %+v err=%v", second, err)
	}
}

func TestLoginAssignsConfiguredRoles(t *testing.T) {
	svc, _ := newSessionService()
	for phone, want := range map[string]enum.Role{vendorPhone: enum.RoleVendor, adminPhone: enum.RoleAdmin} {
		res, err := svc.Login(context.Background(), phone)
		if err != nil || res.User.Role != want {
			t.Errorf("%s: got %+v err=%v, want role %s", phone, res, err, want)
		}
	}
}

func TestLoginRejectsInvalidPhone(t *testing.T) {
	svc, _ := newSessionService()
	if _, err := svc.Login(context.Background(), "nope"); err == nil {
		t.Fatal("invalid phone must be rejected")
	}
}

// statusRepo returns a stored user with a fixed status, like a database row.
type statusRepo struct{ status model.UserStatus }

func (r statusRepo) FindOrCreateByPhone(_ context.Context, u *model.User) (*model.User, error) {
	cp := *u
	cp.Status = r.status
	return &cp, nil
}

var _ repository.UserRepository = statusRepo{}

func TestLoginRefusesInactiveAccounts(t *testing.T) {
	tokens := token.NewJWTManager("test-secret-test-secret-test-secret-123", "test", time.Hour)
	for _, status := range []model.UserStatus{model.UserStatusSuspended, model.UserStatusDeleted} {
		svc := NewSessionService(statusRepo{status: status}, tokens, nil, discard)
		res, err := svc.Login(context.Background(), customerPhone)
		var appErr exception.AppError
		if res != nil || !errors.As(err, &appErr) || appErr.Code() != exception.CodeForbidden {
			t.Errorf("%s: got %+v, %v; want 403", status, res, err)
		}
	}

	svc := NewSessionService(statusRepo{status: model.UserStatusActive}, tokens, nil, discard)
	if _, err := svc.Login(context.Background(), customerPhone); err != nil {
		t.Fatalf("active user must log in: %v", err)
	}
}
