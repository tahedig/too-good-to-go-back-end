package bootstrap

const (
	AppName = "too-good-to-go-backend"

	EnvDevelopment = "development"
	EnvStaging     = "staging"
	EnvProduction  = "production"

	HeaderRequestID = "X-Request-ID"

	CtxKeyRequestID = "request_id"
	CtxKeyUserID    = "user_id" // set by the JWT middleware once auth exists

	HealthPath = "/health"

	SMSProviderFake = "fake"
)
