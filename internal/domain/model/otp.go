package model

import "time"

// OTP is a one-time password issued to a phone number. Only a salted hash of
// the code is kept.
type OTP struct {
	Phone     string // E.164
	CodeHash  string
	Salt      string
	CreatedAt time.Time // when the code was sent; drives the resend cooldown
	ExpiresAt time.Time
	Attempts  int
	Used      bool
}

func (o *OTP) Expired(now time.Time) bool { return !now.Before(o.ExpiresAt) }
