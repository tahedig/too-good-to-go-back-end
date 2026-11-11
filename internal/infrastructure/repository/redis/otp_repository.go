package redis

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/repository"
)

// Each OTP is one hash at <prefix>otp:<phone>. Times are stored as Unix
// milliseconds because Lua numbers are doubles and cannot hold nanoseconds
// exactly. Every mutation is a Lua script, so it is atomic across instances.
//
// Scripts only use commands available since Redis 2.6 (HMSET, not multi-field
// HSET) so old local servers keep working.

// saveScript stores the OTP unless the previous one is still inside the
// cooldown, in which case it returns the previous one untouched.
// KEYS[1]=key ARGV: phone, code_hash, salt, created_at, expires_at, cooldown_ms, ttl_ms
var saveScript = goredis.NewScript(`
local prev = redis.call('HGET', KEYS[1], 'created_at')
local cooldown = tonumber(ARGV[6])
if prev and cooldown > 0 and tonumber(ARGV[4]) - tonumber(prev) < cooldown then
  return redis.call('HGETALL', KEYS[1])
end
redis.call('DEL', KEYS[1])
redis.call('HMSET', KEYS[1],
  'phone', ARGV[1], 'code_hash', ARGV[2], 'salt', ARGV[3],
  'created_at', ARGV[4], 'expires_at', ARGV[5], 'attempts', 0, 'used', 0)
redis.call('PEXPIRE', KEYS[1], ARGV[7])
return false
`)

// recordAttemptScript increments the attempt counter and returns the updated hash.
var recordAttemptScript = goredis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then
  return false
end
redis.call('HINCRBY', KEYS[1], 'attempts', 1)
return redis.call('HGETALL', KEYS[1])
`)

// markUsedScript flips used from 0 to 1 exactly once.
var markUsedScript = goredis.NewScript(`
local used = redis.call('HGET', KEYS[1], 'used')
if not used or used == '1' then
  return 0
end
redis.call('HSET', KEYS[1], 'used', 1)
return 1
`)

type otpRepository struct {
	client goredis.UniversalClient
	prefix string
}

func NewOTPRepository(client goredis.UniversalClient, keyPrefix string) repository.OTPRepository {
	return &otpRepository{client: client, prefix: keyPrefix + "otp:"}
}

func (r *otpRepository) key(phone string) string { return r.prefix + phone }

func (r *otpRepository) Save(ctx context.Context, otp *model.OTP, cooldown time.Duration) (*model.OTP, error) {
	// Keep the key at least as long as the cooldown, so a code that expires
	// quickly cannot be used to skip the resend limit.
	ttl := max(otp.ExpiresAt.Sub(otp.CreatedAt), cooldown, time.Second)

	res, err := saveScript.Run(ctx, r.client, []string{r.key(otp.Phone)},
		otp.Phone, otp.CodeHash, otp.Salt,
		otp.CreatedAt.UnixMilli(), otp.ExpiresAt.UnixMilli(),
		cooldown.Milliseconds(), ttl.Milliseconds(),
	).Result()
	if errors.Is(err, goredis.Nil) {
		return nil, nil // saved
	}
	if err != nil {
		return nil, fmt.Errorf("redis save otp: %w", err)
	}
	return decodeOTP(res)
}

func (r *otpRepository) RecordAttempt(ctx context.Context, phone string) (*model.OTP, error) {
	res, err := recordAttemptScript.Run(ctx, r.client, []string{r.key(phone)}).Result()
	if errors.Is(err, goredis.Nil) {
		return nil, nil // no OTP (never requested or expired)
	}
	if err != nil {
		return nil, fmt.Errorf("redis record otp attempt: %w", err)
	}
	return decodeOTP(res)
}

func (r *otpRepository) MarkUsed(ctx context.Context, phone string) (bool, error) {
	n, err := markUsedScript.Run(ctx, r.client, []string{r.key(phone)}).Int()
	if err != nil {
		return false, fmt.Errorf("redis mark otp used: %w", err)
	}
	return n == 1, nil
}

func (r *otpRepository) Delete(ctx context.Context, phone string) error {
	if err := r.client.Del(ctx, r.key(phone)).Err(); err != nil {
		return fmt.Errorf("redis delete otp: %w", err)
	}
	return nil
}

// decodeOTP turns a Lua HGETALL reply (a flat field/value list) into a model.
func decodeOTP(res any) (*model.OTP, error) {
	list, ok := res.([]any)
	if !ok || len(list)%2 != 0 {
		return nil, fmt.Errorf("redis otp: unexpected reply %T", res)
	}
	fields := make(map[string]string, len(list)/2)
	for i := 0; i < len(list); i += 2 {
		k, _ := list[i].(string)
		v, _ := list[i+1].(string)
		fields[k] = v
	}

	ms := func(name string) (time.Time, error) {
		n, err := strconv.ParseInt(fields[name], 10, 64)
		if err != nil {
			return time.Time{}, fmt.Errorf("redis otp: bad %s %q", name, fields[name])
		}
		return time.UnixMilli(n), nil
	}
	createdAt, err := ms("created_at")
	if err != nil {
		return nil, err
	}
	expiresAt, err := ms("expires_at")
	if err != nil {
		return nil, err
	}
	attempts, err := strconv.Atoi(fields["attempts"])
	if err != nil {
		return nil, fmt.Errorf("redis otp: bad attempts %q", fields["attempts"])
	}

	return &model.OTP{
		Phone:     fields["phone"],
		CodeHash:  fields["code_hash"],
		Salt:      fields["salt"],
		CreatedAt: createdAt,
		ExpiresAt: expiresAt,
		Attempts:  attempts,
		Used:      fields["used"] == "1",
	}, nil
}
