package dto

// CreateStoreRequestInput represents the payload to submit a new store or branch request.
type CreateStoreRequestInput struct {
	StoreName  string `json:"store_name" example:"نان صبحگاهی - شعبه شریعتی"`
	CategoryID *int16 `json:"category_id,omitempty" example:"1"`
	OwnerName  string `json:"owner_name,omitempty" example:"آیدا کریمی"`
	Address    string `json:"address" example:"تهران، خیابان شریعتی"`
}

// StoreRequestResponse represents a store request record.
type StoreRequestResponse struct {
	PublicID     string `json:"public_id" example:"req-550e8400-e29b-41d4-a716-446655440000"`
	StoreName    string `json:"store_name" example:"نان صبحگاهی - شعبه شریعتی"`
	CategoryID   *int16 `json:"category_id,omitempty" example:"1"`
	CategoryName string `json:"category_name,omitempty" example:"نانوایی"`
	OwnerName    string `json:"owner_name,omitempty" example:"آیدا کریمی"`
	Date         string `json:"date" example:"۲۷ مهر ۱۴۰۵"`
	Address      string `json:"address" example:"تهران، خیابان شریعتی"`
	Status       string `json:"status" example:"pending_review"`
	StatusFa     string `json:"status_fa" example:"در انتظار بررسی"`
	Notice       string `json:"notice" example:"این شعبه پس از تأیید، امکان انتشار محصول خواهد داشت."`
	CreatedAt    string `json:"created_at"`
}

// StoreRequestListResponse represents the list of store creation requests.
type StoreRequestListResponse struct {
	Requests []StoreRequestResponse `json:"requests"`
}
