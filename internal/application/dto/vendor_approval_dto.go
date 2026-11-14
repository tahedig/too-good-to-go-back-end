package dto

import "time"

// VendorResponse is a store as the admin reviews it. Approval is per store,
// so id is the store's ID. Times are UTC.
type VendorResponse struct {
	ID          string    `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	StoreName   string    `json:"store_name" example:"نان صبحگاهی"`
	OwnerName   string    `json:"owner_name" example:"سمیرا محمدی"`
	Category    string    `json:"category" example:"نانوایی"`
	Address     string    `json:"address" example:"خیابان شریعتی، پلاک ۲۴"`
	Phone       string    `json:"phone" example:"+989121111111"`
	Email       *string   `json:"email" example:"null"` // always null: accounts have no email
	Description string    `json:"description" example:"نان و شیرینی پایان روز"`
	ImageURL    *string   `json:"image_url" example:"https://example.com/store.jpg"`
	SubmittedAt time.Time `json:"submitted_at" example:"2026-10-19T07:10:00Z"`
	// Status: pending, approved, rejected or suspended.
	Status          string     `json:"status" example:"pending"`
	StatusUpdatedAt *time.Time `json:"status_updated_at" example:"2026-10-20T09:00:00Z"`
	// Decision is the latest rejection or suspension; null otherwise.
	Decision *VendorDecisionResponse `json:"decision"`
}

type VendorDecisionResponse struct {
	// Type is rejection or suspension.
	Type       string    `json:"type" example:"rejection"`
	ReasonCode string    `json:"reason_code" example:"unverifiable_address"`
	Message    string    `json:"message" example:"نشانی تحویل قابل تأیید نبود. لطفاً نشانی را اصلاح کنید."`
	DecidedAt  time.Time `json:"decided_at" example:"2026-10-16T18:00:00Z"`
	DecidedBy  string    `json:"decided_by" example:"مدیر سامانه"`
}

type VendorListResponse struct {
	Items    []VendorResponse `json:"items"`
	Total    int              `json:"total" example:"37"`
	Page     int              `json:"page" example:"1"`
	PageSize int              `json:"page_size" example:"20"`
	// Counts per status over all stores, ignoring the filters.
	Counts VendorCountsResponse `json:"counts"`
}

type VendorCountsResponse struct {
	Pending   int `json:"pending" example:"3"`
	Approved  int `json:"approved" example:"30"`
	Rejected  int `json:"rejected" example:"2"`
	Suspended int `json:"suspended" example:"2"`
}

// VendorDecisionRequest is the body of reject and suspend. Any other field
// (e.g. status) is ignored.
type VendorDecisionRequest struct {
	ReasonCode string `json:"reason_code" example:"unverifiable_address"`
	// Message for the vendor: 20 to 500 characters after trimming.
	Message string `json:"message" example:"نشانی تحویل قابل تأیید نبود. لطفاً نشانی را اصلاح کنید."`
}
