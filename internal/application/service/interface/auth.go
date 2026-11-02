package serviceinterface

import (
	"context"
	"time"
)

// OTPRequestResult is returned after a code was issued and sent.
type OTPRequestResult struct {
	Phone string // normalized, E.164
	TTL   time.Duration
	// DebugCode is the plain code, set only when OTP_EXPOSE_CODE is enabled.
	DebugCode string
}

// OTPVerifyResult is returned after a code was verified and consumed.
type OTPVerifyResult struct {
	Phone string // normalized, E.164
}

type AuthService interface {
	// RequestOTP issues a new OTP for phone (replacing any previous one) and
	// sends it by SMS.
	RequestOTP(ctx context.Context, phone string) (*OTPRequestResult, error)
	// VerifyOTP checks code against the active OTP and consumes it on success.
	VerifyOTP(ctx context.Context, phone, code string) (*OTPVerifyResult, error)
}
