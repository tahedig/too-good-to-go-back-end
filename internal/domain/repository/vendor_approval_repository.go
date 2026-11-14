package repository

import (
	"context"
	"errors"

	"github.com/tahedig/too-good-to-go/backend/internal/domain/enum"
	"github.com/tahedig/too-good-to-go/backend/internal/domain/model"
)

var (
	// ErrStoreNotFound means no (non-deleted) store has the given public ID.
	ErrStoreNotFound = errors.New("store not found")
	// ErrStoreStatusChanged means the store was not in the expected status
	// any more, e.g. another admin decided first.
	ErrStoreStatusChanged = errors.New("store status changed")
)

// VendorStoreQuery selects one page of stores for the admin list.
type VendorStoreQuery struct {
	Statuses []enum.StoreStatus // nil or empty = all
	Search   string             // case-insensitive substring; empty = no search
	Offset   int
	Limit    int
}

// VendorStorePage is a page of stores plus the totals the admin page shows.
type VendorStorePage struct {
	Items []model.VendorStore
	// Total counts every store matching the query's filters.
	Total int
	// Counts counts every store per status, ignoring the filters.
	Counts map[enum.StoreStatus]int
}

type VendorApprovalRepository interface {
	// List returns stores newest submitted first.
	List(ctx context.Context, q VendorStoreQuery) (*VendorStorePage, error)
	// Get returns one store, or ErrStoreNotFound.
	Get(ctx context.Context, storeID string) (*model.VendorStore, error)
	// Transition moves the store from t.From to t.To and records the
	// decision, atomically. It returns ErrStoreStatusChanged when the store
	// is no longer in t.From and ErrStoreNotFound when it does not exist.
	Transition(ctx context.Context, t model.StoreTransition) (*model.VendorStore, error)
}
