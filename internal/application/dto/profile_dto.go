package dto

// ProfileResponse contains the verified phone and editable display name.
type ProfileResponse struct {
	UserID      string `json:"user_id" example:"5f0c6d1e-8b7a-4c1e-9a3f-2d4b6c8e0a12"`
	DisplayName string `json:"display_name" example:"Sam Example"`
	Phone       string `json:"phone" example:"+989123456789"`
}

// UpdateProfileRequest contains the only customer-editable profile field.
type UpdateProfileRequest struct {
	DisplayName string `json:"display_name"`
}
