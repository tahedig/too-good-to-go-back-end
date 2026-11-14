package model

import (
	"time"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
)

// VendorStore is a store as the admin reviews it: the store, its owner and
// the latest decision. Approval is per store (stores.status). ID is the
// store's public ID.
type VendorStore struct {
	ID          string
	StoreName   string
	OwnerName   string
	Category    string
	Address     string
	Phone       string
	Description string
	ImageURL    *string
	SubmittedAt time.Time
	Status      enum.StoreStatus
	// StatusUpdatedAt is the time of the latest decision; nil while the
	// store has never been decided on.
	StatusUpdatedAt *time.Time
	// Decision is the latest rejection or suspension. It is nil unless the
	// store is currently rejected or suspended.
	Decision *StoreDecision
}

// StoreDecision is why a store was rejected or suspended.
type StoreDecision struct {
	Action     enum.StoreAction // reject or suspend
	ReasonCode string
	Message    string
	DecidedAt  time.Time
	DecidedBy  string // the admin's name, or phone when they have no name
}

// StoreTransition is one admin decision to apply atomically.
type StoreTransition struct {
	StoreID     string // public ID
	Action      enum.StoreAction
	From, To    enum.StoreStatus
	ReasonCode  string // empty for approve / reactivate
	Message     string
	AdminUserID string // public ID of the deciding admin, from the token
}
