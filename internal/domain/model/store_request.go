package model

import "time"

// StoreRequest represents a request submitted by a vendor to create a new store or branch.
type StoreRequest struct {
	ID           int64
	PublicID     string
	VendorID     int64
	StoreName    string
	CategoryID   *int16
	CategoryName string
	OwnerName    string
	Address      string
	Status       string // "pending_review", "approved", "rejected"
	StatusFa     string // "در انتظار بررسی", "تأییدشده", "رد شده"
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
