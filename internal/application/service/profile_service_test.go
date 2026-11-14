package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/exception"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/infrastructure/repository/memory"
)

func TestProfileServiceGetAndUpdate(t *testing.T) {
	ctx := context.Background()
	users := memory.NewUserRepository()
	_, err := users.FindOrCreateByPhone(ctx, &model.User{
		ID: "customer-1", Phone: "+989123456789", Role: enum.RoleCustomer, FullName: "Old Name",
	})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewProfileService(users)

	got, err := svc.Get(ctx, "customer-1")
	if err != nil || got.FullName != "Old Name" || got.Phone != "+989123456789" {
		t.Fatalf("Get() = %+v, %v", got, err)
	}
	got, err = svc.UpdateFullName(ctx, "customer-1", "  New Name  ")
	if err != nil || got.FullName != "New Name" || got.Phone != "+989123456789" ||
		got.Role != enum.RoleCustomer {
		t.Fatalf("UpdateFullName() = %+v, %v", got, err)
	}
}

func TestProfileServiceRejectsInvalidDisplayNames(t *testing.T) {
	ctx := context.Background()
	users := memory.NewUserRepository()
	_, _ = users.FindOrCreateByPhone(ctx, &model.User{
		ID: "customer-1", Phone: "+989123456789", Role: enum.RoleCustomer, FullName: "Original",
	})
	svc := NewProfileService(users)
	for _, name := range []string{" \t ", strings.Repeat("x", maxProfileNameLength+1)} {
		if got, err := svc.UpdateFullName(ctx, "customer-1", name); got != nil {
			t.Errorf("UpdateFullName(%q) returned user %+v", name, got)
		} else {
			var appErr exception.AppError
			if !errors.As(err, &appErr) || appErr.Code() != exception.CodeBadRequest {
				t.Errorf("UpdateFullName(%q) error = %v, want bad request", name, err)
			}
		}
	}
	user, err := svc.Get(ctx, "customer-1")
	if err != nil || user.FullName != "Original" {
		t.Fatalf("invalid update changed the profile: %+v, %v", user, err)
	}
}
