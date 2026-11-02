package exception

import (
	"fmt"
	"net/http"
	"time"
)

const (
	CodeInvalidPhone     = "INVALID_PHONE_NUMBER"
	CodeOTPInvalid       = "OTP_INVALID"
	CodeOTPExpired       = "OTP_EXPIRED"
	CodeOTPAttemptsLimit = "OTP_ATTEMPTS_EXCEEDED"
	CodeOTPResendTooSoon = "OTP_RESEND_TOO_SOON"
)

func NewInvalidPhone() AppError {
	return &appError{
		status:  http.StatusBadRequest,
		code:    CodeInvalidPhone,
		message: "phone number is not valid",
		details: []FieldError{{Field: "phone", Message: "must be a valid phone number, e.g. +989123456789 or 09123456789"}},
	}
}

// NewOTPMalformed is for a code that cannot possibly be correct (wrong length
// or not digits). It does not consume a verification attempt.
func NewOTPMalformed(length int) AppError {
	return NewBadRequest("the verification code is malformed",
		[]FieldError{{Field: "code", Message: fmt.Sprintf("must be exactly %d digits", length)}})
}

// NewOTPInvalid is for a wrong code.
func NewOTPInvalid(remainingAttempts int) AppError {
	return &appError{
		status:  http.StatusBadRequest,
		code:    CodeOTPInvalid,
		message: "the verification code is incorrect",
		details: map[string]int{"remaining_attempts": remainingAttempts},
	}
}

// NewOTPExpired covers expired, already used and never requested codes.
func NewOTPExpired() AppError {
	return &appError{
		status:  http.StatusBadRequest,
		code:    CodeOTPExpired,
		message: "the verification code has expired or was already used; request a new one",
	}
}

func NewOTPAttemptsExceeded() AppError {
	return &appError{
		status:  http.StatusTooManyRequests,
		code:    CodeOTPAttemptsLimit,
		message: "too many incorrect attempts; request a new verification code",
	}
}

// NewOTPResendTooSoon is returned when a new code is requested for the same
// phone before the resend cooldown has elapsed.
func NewOTPResendTooSoon(retryAfter time.Duration) AppError {
	return &appError{
		status:     http.StatusTooManyRequests,
		code:       CodeOTPResendTooSoon,
		message:    "a verification code was sent recently; wait before requesting a new one",
		details:    retryDetails(retryAfter),
		retryAfter: retryAfter,
	}
}
