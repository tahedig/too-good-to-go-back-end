package dto

type RequestOTPRequest struct {
	// Iranian local (09...), 98... or E.164 (+...) format.
	Phone string `json:"phone" example:"09123456789"`
}

type RequestOTPResponse struct {
	Message          string `json:"message" example:"verification code sent"`
	ExpiresInSeconds int    `json:"expires_in_seconds" example:"300"`
	// DebugCode is present only when OTP_EXPOSE_CODE=true (never in production).
	DebugCode string `json:"debug_code,omitempty" example:"123456"`
}

type VerifyOTPRequest struct {
	Phone string `json:"phone" example:"09123456789"`
	Code  string `json:"code" example:"123456"`
}

type VerifyOTPResponse struct {
	Verified bool   `json:"verified" example:"true"`
	Phone    string `json:"phone" example:"+989123456789"`
}
