package model

import (
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
)

type User struct {
	ID        string
	Phone     string // E.164, unique
	Role      enum.Role
	CreatedAt time.Time
}
