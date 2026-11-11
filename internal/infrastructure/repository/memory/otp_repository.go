// Package memory holds in-process repositories used until Postgres/Redis
// implementations exist. State is lost on restart and not shared across instances.
package memory

import (
	"context"
	"sync"
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

type otpRepository struct {
	mu   sync.Mutex
	otps map[string]*model.OTP
}

func NewOTPRepository() repository.OTPRepository {
	return &otpRepository{otps: make(map[string]*model.OTP)}
}

func (r *otpRepository) Save(_ context.Context, otp *model.OTP, cooldown time.Duration) (*model.OTP, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if prev, ok := r.otps[otp.Phone]; ok && cooldown > 0 && otp.CreatedAt.Sub(prev.CreatedAt) < cooldown {
		cp := *prev
		return &cp, nil
	}
	cp := *otp
	r.otps[otp.Phone] = &cp
	return nil, nil
}

func (r *otpRepository) RecordAttempt(_ context.Context, phone string) (*model.OTP, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	otp, ok := r.otps[phone]
	if !ok {
		return nil, nil
	}
	otp.Attempts++
	cp := *otp
	return &cp, nil
}

func (r *otpRepository) MarkUsed(_ context.Context, phone string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	otp, ok := r.otps[phone]
	if !ok || otp.Used {
		return false, nil
	}
	otp.Used = true
	return true, nil
}

func (r *otpRepository) Delete(_ context.Context, phone string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.otps, phone)
	return nil
}
