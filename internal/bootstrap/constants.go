package bootstrap

const (
	AppName = "too-good-to-go-backend"

	EnvDevelopment = "development"
	EnvStaging     = "staging"
	EnvProduction  = "production"

	HeaderRequestID = "X-Request-ID"

	CtxKeyRequestID = "request_id"
	CtxKeyUserID    = "user_id" // set by the JWT auth middleware
	CtxKeyUserRole  = "user_role"

	HealthPath = "/health"

	SMSProviderFake = "fake"

	// DevFrontendOrigin is the Vite dev server, allowed by CORS in
	// development when CORS_ALLOWED_ORIGINS is unset.
	DevFrontendOrigin = "http://localhost:5173"
)
