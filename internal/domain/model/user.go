package model

import (
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
)

// UserStatus mirrors the user_status enum in the database.
type UserStatus string

const (
	UserStatusActive    UserStatus = "active"
	UserStatusSuspended UserStatus = "suspended"
	UserStatusDeleted   UserStatus = "deleted"
)

type User struct {
	// ID is the public identifier (users.public_id, a UUID). It is what
	// tokens and API responses carry; the database's numeric key never
	// leaves the repository.
	ID        string
	Phone     string // E.164, unique
	Role      enum.Role
	Status    UserStatus
	CreatedAt time.Time
}

// CanLogin reports whether a session may be issued for this user.
func (u *User) CanLogin() bool {
	return u.Status == "" || u.Status == UserStatusActive
}
