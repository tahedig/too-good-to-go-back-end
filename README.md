# Too Good To Go: Backend

Go + Gin backend, organized with Clean Architecture.

## Run

Requires Go 1.24+.

```bash
go run ./cmd/api
```

That's the only command needed. (On a fresh clone without `go.sum`, run
`go mod tidy` once first and commit the generated `go.sum`.) Configuration comes from environment variables
(see `.env.example`). Every variable has a safe development default, so the
server starts without any setup. To override values locally:

```bash
cp .env.example .env     # Windows: copy .env.example .env
```

Real environment variables always take precedence over `.env`. The `.env` file
is parsed by `godotenv`; application-specific types and constraints are then
validated by the config loader. Invalid values (e.g. `HTTP_PORT=abc`) stop
startup with an error listing every bad variable.

Check it is up:

```bash
curl -i http://localhost:8080/health
```

Run tests:

```bash
go test ./...
```

The dependency graph is generated with Google Wire. After changing providers in
`internal/injector/wire.go`, regenerate the checked-in implementation with:

```bash
go generate ./internal/injector
```

Build a binary with a version stamp:

```bash
go build -ldflags "-X main.version=v0.1.0" -o bin/api ./cmd/api
```

## Configuration

| Variable | Default | Description |
|---|---|---|
| `APP_ENV` | `development` | `development`, `staging` or `production` |
| `HTTP_HOST` | (all interfaces) | Listen host |
| `HTTP_PORT` | `8080` | Listen port |
| `HTTP_READ_TIMEOUT` | `10s` | Max time to read a request |
| `HTTP_WRITE_TIMEOUT` | `15s` | Max time to write a response |
| `HTTP_IDLE_TIMEOUT` | `60s` | Keep-alive idle timeout |
| `HTTP_SHUTDOWN_TIMEOUT` | `10s` | Grace period for in-flight requests on SIGINT/SIGTERM |
| `HEALTH_CHECK_TIMEOUT` | `2s` | Timeout per dependency check in `/health` |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `HTTP_MAX_BODY_BYTES` | `1048576` | Max request body size (1 KiB to 64 MiB); larger bodies get `413` |
| `OTP_LENGTH` | `6` | Digits in an OTP (4-8) |
| `OTP_TTL` | `5m` | OTP lifetime |
| `OTP_MAX_ATTEMPTS` | `5` | Verification attempts per OTP (1-20) |
| `OTP_RESEND_COOLDOWN` | `1m` | Minimum time between two codes for the same phone |
| `OTP_EXPOSE_CODE` | `false` | Return the code as `debug_code` in the request response, for testing while SMS is faked. **Refused** when `APP_ENV=production` |
| `SMS_PROVIDER` | `fake` | SMS provider; only the sandbox `fake` exists. Outside production it logs the SMS body at debug level |
| `AUTH_RATE_LIMIT` | `20` | Requests per client IP per window on `/auth/otp/*` |
| `AUTH_RATE_WINDOW` | `1m` | Window for `AUTH_RATE_LIMIT` |
| `REDIS_URL` | (none) | `redis://[:password@]host:port/db` (`rediss://` for TLS). Stores OTPs and rate-limit counters. **Required** when `APP_ENV` is not `development`; unset in development falls back to process memory |
| `REDIS_KEY_PREFIX` | `tgtg:` | Prefix for every Redis key, so environments can share one Redis |
| `DATABASE_URL` | (none) | `postgres://user:pass@host:5432/db?sslmode=disable`. Stores users and their roles. **Required** when `APP_ENV` is not `development`; unset in development falls back to process memory (users are lost on restart) |
| `JWT_SECRET` | (random in development) | HS256 signing secret, min 32 chars. **Required** when `APP_ENV` is not `development`; unset in development generates a random one per start |
| `JWT_ISSUER` | `too-good-to-go-backend` | `iss` claim, checked on every request |
| `JWT_ACCESS_TTL` | `1h` | Access token lifetime |
| `ADMIN_PHONES` | (none) | Comma separated phones that become `admin` on first login |
| `VENDOR_PHONES` | (none) | Comma separated phones that become `vendor` on first login |
| `TRUSTED_PROXIES` | (none) | Comma separated proxy IPs/CIDRs for `X-Forwarded-For` |

No secret or environment specific value is committed: `.env` is git-ignored and
only `.env.example` (placeholders and defaults) is tracked.

## OTP authentication

Login is passwordless: the client requests a one-time code by SMS, then
verifies it. A successful verify logs the user in (creating the account on
first login) and returns a JWT access token.

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/auth/otp/request` | none | `{"phone": "09123456789"}` sends a code. Iranian local, `98...` and E.164 formats are accepted |
| `POST` | `/auth/otp/verify` | none | `{"phone": "...", "code": "123456"}` checks and consumes the code, then returns a session (below) |

Successful verify response:

```json
{
  "verified": true,
  "phone": "+989123456789",
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "token_type": "Bearer",
  "expires_in": 3600,
  "user": { "id": "5f0c6d1e-...", "phone": "+989123456789", "role": "customer" }
}
```

Send the token as `Authorization: Bearer <access_token>`. `expires_in` is in
seconds (`JWT_ACCESS_TTL`); there is no refresh token yet, so log in again
after it expires.

Error codes clients should handle:

| Status | `code` | Meaning |
|---|---|---|
| 400 | `INVALID_PHONE_NUMBER` | Phone cannot be parsed |
| 400 | `BAD_REQUEST` | Body is not JSON, or the code has the wrong length or non-digits (does **not** use up an attempt) |
| 400 | `OTP_INVALID` | Wrong code; `details.remaining_attempts` says how many tries are left |
| 400 | `OTP_EXPIRED` | Code expired, already used, or never requested |
| 429 | `OTP_ATTEMPTS_EXCEEDED` | Too many wrong codes; request a new one |
| 429 | `OTP_RESEND_TOO_SOON` | New code requested within `OTP_RESEND_COOLDOWN` |
| 429 | `TOO_MANY_REQUESTS` | Per-IP limit on `/auth/otp/*` hit |
| 413 | `PAYLOAD_TOO_LARGE` | Body larger than `HTTP_MAX_BODY_BYTES` |

Every `429` carries a `Retry-After` header (seconds) and
`details.retry_after_seconds`.

Abuse protection: OTPs are stored only as salted hashes, attempts are counted
atomically before comparing, codes are single use, and SMS sending is limited
per phone (cooldown) and per IP (rate limit). The rate limiter keys on the
client IP, so set `TRUSTED_PROXIES` correctly behind a reverse proxy. If Redis
is unreachable the rate limiter lets requests through (and logs an error)
rather than blocking all logins; OTP operations fail with 500.

**Storage.** With `REDIS_URL` set, OTPs and rate-limit counters live in Redis
and are shared by every instance:

| Key | Type | TTL |
|---|---|---|
| `<prefix>otp:<phone>` | hash: `code_hash`, `salt`, `created_at`, `expires_at` (Unix ms), `attempts`, `used` | `max(OTP_TTL, OTP_RESEND_COOLDOWN)` |
| `<prefix>ratelimit:auth:<ip>` | counter | `AUTH_RATE_WINDOW` |

Every write is a Lua script, so cooldown, attempt counting and single use stay
atomic across instances. Scripts only use commands from Redis 2.6+, so any
Redis version works. Redis is pinged at startup (a wrong `REDIS_URL` stops the
server) and is reported in `/health` as `checks.redis`.

**SMS delivery** is a sandbox for now (`SMS_PROVIDER=fake`): nothing leaves the
server. To test the flow, either set `LOG_LEVEL=debug` and read the code from
the `fake sms sent` log line, or set `OTP_EXPOSE_CODE=true` and read
`debug_code` from the `/auth/otp/request` response. A real provider only needs
to implement `serviceinterface.SMSSender` in `internal/infrastructure/sms` and
be added to `newSMSSender` in `internal/injector`.

## Health endpoint

`GET /health` (also `HEAD`)

| Status | When |
|---|---|
| `200 OK` | Service and every registered dependency check pass |
| `503 Service Unavailable` | A dependency check fails or times out, or the server is shutting down |

```json
{
  "status": "ok",
  "version": "dev",
  "timestamp": "2026-10-25T10:00:00Z",
  "checks": { "postgres": { "status": "ok", "latency_ms": 3 } }
}
```

`checks` appears once dependencies are registered. To add one, implement
`serviceinterface.HealthChecker` (`Name()`, `Check(ctx)`) in the infrastructure
layer and pass it to `service.NewHealthService` in `internal/injector`.
Failure reasons are logged, never returned, because `/health` is public.

## Authentication and RBAC (JWT)

Protected routes require an HS256 access token:
`Authorization: Bearer <access_token>`.

| Method | Path | Access | Description |
|---|---|---|---|
| `GET` | `/auth/me` | any role | User id and role proven by the token |
| `GET` | `/test/customer` | `customer` | RBAC test route |
| `GET` | `/test/vendor` | `vendor` | RBAC test route |
| `GET` | `/test/admin` | `admin` | RBAC test route |
| any | `/vendor/stores...`, `/vendor/store-requests...` | `vendor` | Vendor store management |

A missing, malformed, expired or wrongly signed token, a wrong issuer or
algorithm, or an unknown role gets `401 UNAUTHORIZED`; a valid token with the
wrong role gets `403 FORBIDDEN`.

Tokens are issued by `POST /auth/otp/verify`, which calls
`SessionService.Login` once the code is accepted. It loads or creates the user;
new users are `customer` unless their number is in `ADMIN_PHONES` /
`VENDOR_PHONES`. Roles of existing users are never changed on login.

### Users in PostgreSQL

With `DATABASE_URL` set, users live in the `users`, `roles` and `user_roles`
tables from `migrations/`. Apply them before starting the server:

```bash
make migrate-up DB_URL="postgres://app:app@localhost:5432/tgtg?sslmode=disable"
```

The server checks the schema at startup and refuses to start with
`database schema is missing or outdated` if the migrations were not applied.
`/health` reports `postgres` once it is connected.

- **IDs.** Tokens and API responses carry `users.public_id` (UUID). The numeric
  `users.id` never leaves the database.
- **First login** inserts the user and grants one role: `customer`,
  `vendor_owner` (for `VENDOR_PHONES`) or `admin` (for `ADMIN_PHONES`). It is a
  single `INSERT ... ON CONFLICT (phone)`, so parallel first logins create one
  user. `last_login_at` is updated on every successful login.
- **Roles.** The token holds one role: the strongest the user has in
  `user_roles` (`admin` > `vendor_owner`/`vendor_staff` → `vendor` >
  `customer`). `support` has no API role yet and is ignored. Roles granted in
  the database apply from the next login.
- **Status.** `suspended` or `deleted` users get `403 FORBIDDEN`
  (`this account is not active`) and no token.

OTP codes stay in Redis (the `otp_codes` table is not used): they are
short-lived and Redis expires them on its own.

Integration tests run against a real database when `TEST_DATABASE_URL` is set
(each test creates and drops its own schema, so any empty database works);
otherwise they are skipped:

```bash
TEST_DATABASE_URL="postgres://app:app@localhost:5432/tgtg_test?sslmode=disable"   go test ./internal/infrastructure/repository/postgres/
```

To protect a new route:

```go
group := r.Group("/vendor", authMW.Authenticate(), authMW.RequireRoles(enum.RoleVendor))
```

Handlers read the caller with `c.GetString(bootstrap.CtxKeyUserID)` and
`c.GetString(bootstrap.CtxKeyUserRole)`.

## Error response format

Every error from every endpoint (including 404, 405 and recovered panics) has
the same shape:

```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "route not found",
    "details": null,
    "request_id": "3f2a9c..."
  }
}
```

`details` is omitted when empty; for validation errors it holds a list of
`{"field": "...", "message": "..."}`. Clients should switch on `code`, not on
`message`. 5xx responses never include internal error text.

In handlers, never write error bodies yourself. Return a typed error from
`internal/domain/exception` and attach it:

```go
if err != nil {
    _ = c.Error(err) // or the abortWithError helper in handler/helpers.go
    c.Abort()
    return
}
```

Errors that are not an `exception.AppError` become `500 INTERNAL_SERVER_ERROR`.

## Logging

All logs are single-line JSON on stdout (`log/slog`). Each request produces one
`http request` entry with `request_id`, `method`, `path`, `route`, `status`,
`latency_ms`, `client_ip`, `user_agent`, and `user_id` once auth sets it.
`/health` hits are logged at debug level to avoid noise. Keys containing
`password`, `token`, `secret`, `otp`, `authorization`, `cookie` and similar are
masked as `[REDACTED]`.

The request id comes from an incoming `X-Request-ID` header (if well formed) or
is generated, and is returned in the response header and in error bodies. To
log with it from any layer:

```go
logger.FromContext(ctx, s.log).Info("offer reserved", "offer_id", id)
```

## Project layout

```
cmd/api/                  entrypoint: config, logger, server, graceful shutdown
internal/
  bootstrap/              env config and constants
  domain/                 models, enums, repository interfaces, typed exceptions
  application/            DTOs, service interfaces and implementations, workers
  infrastructure/         Postgres/Redis repositories, storage, realtime, ...
  presentation/           handlers, middleware, router
  injector/               Wire injector and generated dependency wiring
pkg/                      reusable helpers with no project dependencies (logger, ...)
docs/                     Swagger output
```

Dependency rule: `presentation → application → domain`, and `infrastructure`
implements interfaces declared in `domain`/`application`. Inner layers never
import outer ones.
