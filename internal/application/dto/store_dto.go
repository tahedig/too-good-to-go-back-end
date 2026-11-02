package dto

type UpdateStoreRequest struct {
	Name               string `json:"name" example:"My Store"`
	Biography          string `json:"biography" example:"Fresh food and groceries"`
	PickupAddress      string `json:"pickup_address" example:"Tehran, Example Street"`
	PickupInstructions string `json:"pickup_instructions" example:"Ring the bell when you arrive"`
}

type StoreLocationResponse struct {
	Latitude  float64 `json:"latitude" example:"35.6892"`
	Longitude float64 `json:"longitude" example:"51.3890"`
}

type StoreResponse struct {
	PublicID           string                `json:"public_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name               string                `json:"name"`
	Biography          string                `json:"biography"`
	Location           StoreLocationResponse `json:"location"`
	PickupAddress      string                `json:"pickup_address"`
	PickupInstructions string                `json:"pickup_instructions"`
	RatingAvg          float64               `json:"rating_avg"`
	TrustScore         float64               `json:"trust_score"`
	ApprovalStatus     string                `json:"approval_status"`
	AvailabilityStatus string                `json:"availability_status"`
	CreatedAt          string                `json:"created_at"`
	UpdatedAt          string                `json:"updated_at"`
}

type StoreListResponse struct {
	Stores []StoreResponse `json:"stores"`
}
