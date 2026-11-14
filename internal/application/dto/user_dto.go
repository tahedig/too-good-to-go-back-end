package dto

// MeResponse is what the token itself proves; no database lookup is needed.
type MeResponse struct {
	UserID string `json:"user_id" example:"5f0c6d1e-8b7a-4c1e-9a3f-2d4b6c8e0a12"`
	Role   string `json:"role" enums:"customer,vendor,admin" example:"customer"`
}
