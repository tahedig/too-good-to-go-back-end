package repository

import (
	"context"
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
)

// OTPRepository stores at most one active OTP per phone number. The mutating
// methods must be atomic so concurrent verifications cannot bypass the
// attempt limit or reuse a code.
type OTPRepository interface {
	// Save stores otp, replacing any previous OTP for the same phone, unless
	// that previous OTP was created less than cooldown before otp.CreatedAt.
	// In that case nothing is stored and the blocking OTP is returned so the
	// caller can tell the client how long to wait. A zero cooldown disables
	// the check.
	Save(ctx context.Context, otp *model.OTP, cooldown time.Duration) (blockedBy *model.OTP, err error)
	// RecordAttempt atomically increments the attempt counter and returns
	// the updated copy, or nil if there is no OTP for phone.
	RecordAttempt(ctx context.Context, phone string) (*model.OTP, error)
	// MarkUsed atomically flags the OTP as used. It returns false if the
	// OTP is missing or was already used.
	MarkUsed(ctx context.Context, phone string) (bool, error)
	Delete(ctx context.Context, phone string) error
}
