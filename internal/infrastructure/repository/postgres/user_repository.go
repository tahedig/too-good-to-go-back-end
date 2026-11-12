// Package postgres holds the PostgreSQL-backed repositories. The schema lives
// in /migrations and is applied with `make migrate-up`.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

// Role codes in the roles table (seeded by migration 000001). The table also
// has "support", which has no application role yet.
const (
	roleCodeCustomer    = "customer"
	roleCodeVendorOwner = "vendor_owner"
	roleCodeVendorStaff = "vendor_staff"
	roleCodeAdmin       = "admin"
)

// roleCodeFor is the database role a new user with an application role gets.
var roleCodeFor = map[enum.Role]string{
	enum.RoleCustomer: roleCodeCustomer,
	enum.RoleVendor:   roleCodeVendorOwner,
	enum.RoleAdmin:    roleCodeAdmin,
}

// appRoleOf maps database roles onto the application roles a token can carry.
// Codes missing here (support) are ignored.
var appRoleOf = map[string]enum.Role{
	roleCodeCustomer:    enum.RoleCustomer,
	roleCodeVendorOwner: enum.RoleVendor,
	roleCodeVendorStaff: enum.RoleVendor,
	roleCodeAdmin:       enum.RoleAdmin,
}

// roleRank orders application roles; a user with several roles is given the
// strongest one in the token.
var roleRank = map[enum.Role]int{enum.RoleCustomer: 1, enum.RoleVendor: 2, enum.RoleAdmin: 3}

type userRepository struct {
	pool *pgxpool.Pool
}

// NewUserRepository stores users in the users and user_roles tables.
func NewUserRepository(pool *pgxpool.Pool) repository.UserRepository {
	return &userRepository{pool: pool}
}

// upsertUserSQL inserts the user, or locks and returns the existing row when
// the phone is taken. ON CONFLICT makes it atomic: parallel first logins wait
// on the unique index and then see the committed row. xmax = 0 is true only
// for a freshly inserted row. last_login_at is only touched for active users.
const upsertUserSQL = `
INSERT INTO users (public_id, phone, last_login_at)
VALUES ($1::uuid, $2, now())
ON CONFLICT (phone) DO UPDATE
    SET last_login_at = CASE WHEN users.status = 'active' THEN now() ELSE users.last_login_at END
RETURNING id, public_id::text, phone, status::text, created_at, (xmax = 0) AS inserted`

const grantRoleSQL = `
INSERT INTO user_roles (user_id, role_id)
SELECT $1, id FROM roles WHERE code = $2
ON CONFLICT DO NOTHING`

const userRolesSQL = `
SELECT r.code
FROM user_roles ur
JOIN roles r ON r.id = ur.role_id
WHERE ur.user_id = $1`

// FindOrCreateByPhone returns the user with newUser.Phone, creating it with
// newUser's ID (as public_id) and role if the phone is new. The roles of an
// existing user are never changed here.
func (r *userRepository) FindOrCreateByPhone(ctx context.Context, newUser *model.User) (*model.User, error) {
	roleCode, ok := roleCodeFor[newUser.Role]
	if !ok {
		return nil, fmt.Errorf("unknown role %q", newUser.Role)
	}

	var user *model.User
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var (
			dbID      int64
			u         model.User
			status    string
			createdAt time.Time
			inserted  bool
		)
		if err := tx.QueryRow(ctx, upsertUserSQL, newUser.ID, newUser.Phone).
			Scan(&dbID, &u.ID, &u.Phone, &status, &createdAt, &inserted); err != nil {
			return fmt.Errorf("upsert user: %w", err)
		}
		u.Status = model.UserStatus(status)
		u.CreatedAt = createdAt.UTC()

		if inserted {
			tag, err := tx.Exec(ctx, grantRoleSQL, dbID, roleCode)
			if err != nil {
				return fmt.Errorf("grant role %s: %w", roleCode, err)
			}
			if tag.RowsAffected() != 1 {
				return fmt.Errorf("role %q is missing from the roles table; run the migrations", roleCode)
			}
		}

		role, err := highestRole(ctx, tx, dbID)
		if err != nil {
			return err
		}
		u.Role = role
		user = &u
		return nil
	})
	if err != nil {
		return nil, err
	}
	return user, nil
}

func highestRole(ctx context.Context, tx pgx.Tx, userID int64) (enum.Role, error) {
	rows, err := tx.Query(ctx, userRolesSQL, userID)
	if err != nil {
		return "", fmt.Errorf("load user roles: %w", err)
	}
	codes, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return "", fmt.Errorf("load user roles: %w", err)
	}

	// A user without any mapped role (none granted, or only support) is a
	// customer: the least privileged role.
	best := enum.RoleCustomer
	for _, code := range codes {
		if role, ok := appRoleOf[code]; ok && roleRank[role] > roleRank[best] {
			best = role
		}
	}
	return best, nil
}

// ErrSchemaMissing means the migrations have not been applied.
var ErrSchemaMissing = errors.New("database schema is missing or outdated: apply the migrations with `make migrate-up`")

// VerifySchema checks that the tables and seeded roles the repositories rely
// on exist, so a database without migrations fails at startup with a clear
// message instead of on the first login.
func VerifySchema(ctx context.Context, pool *pgxpool.Pool) error {
	// Tables first: a query naming a missing table fails to parse, so the
	// roles count can only run once the tables are known to exist.
	var tablesOK bool
	err := pool.QueryRow(ctx, `
SELECT to_regclass('users') IS NOT NULL
   AND to_regclass('roles') IS NOT NULL
   AND to_regclass('user_roles') IS NOT NULL`).Scan(&tablesOK)
	if err != nil {
		return fmt.Errorf("verify database schema: %w", err)
	}
	if !tablesOK {
		return ErrSchemaMissing
	}

	var roles int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM roles WHERE code = ANY($1)`,
		[]string{roleCodeCustomer, roleCodeVendorOwner, roleCodeAdmin}).Scan(&roles); err != nil {
		return fmt.Errorf("verify database schema: %w", err)
	}
	if roles != 3 {
		return ErrSchemaMissing
	}
	return nil
}
