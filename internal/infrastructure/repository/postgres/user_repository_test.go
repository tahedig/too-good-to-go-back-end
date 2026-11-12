package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/pkg/security"
)

// newTestPool connects to TEST_DATABASE_URL inside a fresh schema, so tests
// never touch real data and can run in parallel with other runs. The schema
// is dropped afterwards. With migrate=true the real migration files are
// applied to it first.
func newTestPool(t *testing.T, migrate bool) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping PostgreSQL integration test")
	}
	ctx := context.Background()

	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	schema := fmt.Sprintf("test_users_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") })

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	if migrate {
		files, err := filepath.Glob(filepath.Join("..", "..", "..", "..", "migrations", "*.up.sql"))
		if err != nil || len(files) == 0 {
			t.Fatalf("no migrations found: %v", err)
		}
		sort.Strings(files)
		for _, f := range files {
			sql, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, string(sql)); err != nil {
				t.Fatalf("apply %s: %v", filepath.Base(f), err)
			}
		}
	}
	return pool
}

func newUser(t *testing.T, phone string, role enum.Role) *model.User {
	t.Helper()
	id, err := security.NewID()
	if err != nil {
		t.Fatal(err)
	}
	return &model.User{ID: id, Phone: phone, Role: role, Status: model.UserStatusActive}
}

func dbRoles(t *testing.T, pool *pgxpool.Pool, publicID string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
SELECT r.code FROM user_roles ur JOIN roles r ON r.id = ur.role_id
JOIN users u ON u.id = ur.user_id WHERE u.public_id = $1::uuid ORDER BY r.code`, publicID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatal(err)
		}
		out = append(out, c)
	}
	return out
}

func TestVerifySchema(t *testing.T) {
	ctx := context.Background()
	if err := VerifySchema(ctx, newTestPool(t, false)); !errors.Is(err, ErrSchemaMissing) {
		t.Fatalf("empty schema: got %v, want ErrSchemaMissing", err)
	}
	if err := VerifySchema(ctx, newTestPool(t, true)); err != nil {
		t.Fatalf("migrated schema: %v", err)
	}
}

func TestPostgresFindOrCreateByPhone(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, true)
	repo := NewUserRepository(pool)

	in := newUser(t, "+989123456789", enum.RoleCustomer)
	first, err := repo.FindOrCreateByPhone(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != in.ID || first.Phone != in.Phone || first.Role != enum.RoleCustomer ||
		first.Status != model.UserStatusActive || first.CreatedAt.IsZero() {
		t.Fatalf("unexpected new user: %+v", first)
	}
	if got := dbRoles(t, pool, first.ID); len(got) != 1 || got[0] != "customer" {
		t.Fatalf("customer role not stored: %v", got)
	}

	// Same phone again, with a different ID and role: the stored user wins.
	again, err := repo.FindOrCreateByPhone(ctx, newUser(t, "+989123456789", enum.RoleAdmin))
	if err != nil || again.ID != first.ID || again.Role != enum.RoleCustomer {
		t.Fatalf("existing user must be returned unchanged: %+v err=%v", again, err)
	}
	if got := dbRoles(t, pool, first.ID); len(got) != 1 {
		t.Fatalf("login must not grant roles to an existing user: %v", got)
	}

	var lastLogin *time.Time
	if err := pool.QueryRow(ctx, `SELECT last_login_at FROM users WHERE public_id = $1::uuid`, first.ID).Scan(&lastLogin); err != nil || lastLogin == nil {
		t.Fatalf("last_login_at not recorded: %v %v", lastLogin, err)
	}
}

func TestPostgresNewUserRoles(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, true)
	repo := NewUserRepository(pool)

	cases := []struct {
		phone  string
		role   enum.Role
		dbCode string
	}{
		{"+989121111111", enum.RoleVendor, "vendor_owner"},
		{"+989122222222", enum.RoleAdmin, "admin"},
	}
	for _, tc := range cases {
		u, err := repo.FindOrCreateByPhone(ctx, newUser(t, tc.phone, tc.role))
		if err != nil || u.Role != tc.role {
			t.Fatalf("%s: got %+v err=%v", tc.role, u, err)
		}
		if got := dbRoles(t, pool, u.ID); len(got) != 1 || got[0] != tc.dbCode {
			t.Fatalf("%s: stored roles %v, want [%s]", tc.role, got, tc.dbCode)
		}
	}
}

// Roles granted in the database (e.g. by an admin) decide the token role;
// the strongest one wins and unmapped roles are ignored.
func TestPostgresRoleResolution(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, true)
	repo := NewUserRepository(pool)

	cases := []struct {
		name  string
		grant []string
		want  enum.Role
	}{
		{"customer promoted to admin", []string{"admin"}, enum.RoleAdmin},
		{"vendor staff", []string{"vendor_staff"}, enum.RoleVendor},
		{"customer and vendor owner", []string{"vendor_owner"}, enum.RoleVendor},
		{"support only adds nothing", []string{"support"}, enum.RoleCustomer},
	}
	for i, tc := range cases {
		phone := fmt.Sprintf("+98912000000%d", i)
		u, err := repo.FindOrCreateByPhone(ctx, newUser(t, phone, enum.RoleCustomer))
		if err != nil {
			t.Fatal(err)
		}
		for _, code := range tc.grant {
			if _, err := pool.Exec(ctx, `
INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id FROM users u, roles r WHERE u.public_id = $1::uuid AND r.code = $2`, u.ID, code); err != nil {
				t.Fatal(err)
			}
		}
		got, err := repo.FindOrCreateByPhone(ctx, newUser(t, phone, enum.RoleCustomer))
		if err != nil || got.Role != tc.want {
			t.Errorf("%s: got role %q err=%v, want %q", tc.name, got.Role, err, tc.want)
		}
	}
}

func TestPostgresInactiveUserIsReturnedWithStatus(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, true)
	repo := NewUserRepository(pool)

	u, err := repo.FindOrCreateByPhone(ctx, newUser(t, "+989123456789", enum.RoleCustomer))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET status = 'suspended', last_login_at = NULL WHERE public_id = $1::uuid`, u.ID); err != nil {
		t.Fatal(err)
	}

	got, err := repo.FindOrCreateByPhone(ctx, newUser(t, "+989123456789", enum.RoleCustomer))
	if err != nil || got.ID != u.ID || got.Status != model.UserStatusSuspended || got.CanLogin() {
		t.Fatalf("suspended user: %+v err=%v", got, err)
	}
	var lastLogin *time.Time
	_ = pool.QueryRow(ctx, `SELECT last_login_at FROM users WHERE public_id = $1::uuid`, u.ID).Scan(&lastLogin)
	if lastLogin != nil {
		t.Fatal("a refused login must not update last_login_at")
	}
}

// Parallel first logins with one phone must create exactly one user.
func TestPostgresParallelFirstLogin(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, true)
	repo := NewUserRepository(pool)

	const n = 20
	ids := make([]string, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			u, err := repo.FindOrCreateByPhone(ctx, newUser(t, "+989123456789", enum.RoleCustomer))
			if err == nil {
				ids[i] = u.ID
			}
			errs[i] = err
		}()
	}
	wg.Wait()

	for i := range n {
		if errs[i] != nil {
			t.Fatalf("login %d: %v", i, errs[i])
		}
		if ids[i] != ids[0] {
			t.Fatalf("parallel logins returned different users: %s vs %s", ids[i], ids[0])
		}
	}
	var users, roles int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&users)
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM user_roles`).Scan(&roles)
	if users != 1 || roles != 1 {
		t.Fatalf("got %d users and %d role rows, want 1 and 1", users, roles)
	}
}
