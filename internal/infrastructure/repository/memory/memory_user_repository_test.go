package memory

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

func TestFindOrCreateByPhone(t *testing.T) {
	ctx := context.Background()
	repo := NewUserRepository()

	first, err := repo.FindOrCreateByPhone(ctx, &model.User{ID: "u1", Phone: "+989123456789", Role: enum.RoleVendor})
	if err != nil || first.ID != "u1" || first.Role != enum.RoleVendor {
		t.Fatalf("first call should create the user: %+v err=%v", first, err)
	}

	again, err := repo.FindOrCreateByPhone(ctx, &model.User{ID: "u2", Phone: "+989123456789", Role: enum.RoleAdmin})
	if err != nil || again.ID != "u1" || again.Role != enum.RoleVendor {
		t.Fatalf("existing user must be returned unchanged: %+v err=%v", again, err)
	}

	again.Role = enum.RoleAdmin // callers get copies
	if u, _ := repo.FindOrCreateByPhone(ctx, &model.User{ID: "u3", Phone: "+989123456789"}); u.Role != enum.RoleVendor {
		t.Fatal("mutating a returned user must not change the stored one")
	}
}

func TestFindOrCreateByPhoneIsAtomic(t *testing.T) {
	ctx := context.Background()
	repo := NewUserRepository()

	var (
		wg  sync.WaitGroup
		mu  sync.Mutex
		ids = make(map[string]bool)
	)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			u, err := repo.FindOrCreateByPhone(ctx, &model.User{ID: string(rune('a' + i%26)), Phone: "+989120000000"})
			if err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			ids[u.ID] = true
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	if len(ids) != 1 {
		t.Fatalf("parallel first logins created %d users, want 1", len(ids))
	}
}

func TestFindByID(t *testing.T) {
	ctx := context.Background()
	repo := NewUserRepository()
	if _, err := repo.FindByID(ctx, "u1"); !errors.Is(err, repository.ErrUserNotFound) {
		t.Fatalf("missing user: got %v, want ErrUserNotFound", err)
	}

	_, _ = repo.FindOrCreateByPhone(ctx, &model.User{ID: "u1", Phone: "+989123456789", Role: enum.RoleCustomer, Status: model.UserStatusActive})
	if !repo.SetAccess("u1", enum.RoleVendor, model.UserStatusSuspended) || repo.SetAccess("nobody", enum.RoleAdmin, "") {
		t.Fatal("SetAccess must report whether the user exists")
	}
	u, err := repo.FindByID(ctx, "u1")
	if err != nil || u.Role != enum.RoleVendor || u.Status != model.UserStatusSuspended {
		t.Fatalf("FindByID must see the current state: %+v err=%v", u, err)
	}
	if again, _ := repo.FindOrCreateByPhone(ctx, &model.User{ID: "u2", Phone: "+989123456789"}); again.Status != model.UserStatusSuspended {
		t.Fatal("lookups by phone and by id must share one record")
	}
}
