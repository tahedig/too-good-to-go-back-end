package serviceinterface

import "context"

// SMSSender delivers a text message. Implementations live in
// infrastructure/sms; sprint 1 uses a fake provider, a real one can replace it
// without touching the OTP service.
type SMSSender interface {
	Send(ctx context.Context, phone, body string) error
}
