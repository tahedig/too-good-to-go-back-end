package enum

// StoreStatus mirrors the store_status enum in the database. It is the only
// switch that decides whether a store may publish offers and be shown to
// customers: only approved stores may.
type StoreStatus string

const (
	StoreStatusPending   StoreStatus = "pending"
	StoreStatusApproved  StoreStatus = "approved"
	StoreStatusRejected  StoreStatus = "rejected"
	StoreStatusSuspended StoreStatus = "suspended"
)

// StoreStatuses lists every status, in display order.
var StoreStatuses = []StoreStatus{StoreStatusPending, StoreStatusApproved, StoreStatusRejected, StoreStatusSuspended}

func (s StoreStatus) Valid() bool {
	switch s {
	case StoreStatusPending, StoreStatusApproved, StoreStatusRejected, StoreStatusSuspended:
		return true
	}
	return false
}

// StoreAction is an admin decision on a store.
type StoreAction string

const (
	StoreActionApprove    StoreAction = "approve"
	StoreActionReject     StoreAction = "reject"
	StoreActionSuspend    StoreAction = "suspend"
	StoreActionReactivate StoreAction = "reactivate"
)

type storeTransition struct {
	from, to StoreStatus
	// reasons are the accepted reason codes; nil means the action takes no
	// reason.
	reasons []string
}

var storeTransitions = map[StoreAction]storeTransition{
	StoreActionApprove: {from: StoreStatusPending, to: StoreStatusApproved},
	StoreActionReject: {from: StoreStatusPending, to: StoreStatusRejected, reasons: []string{
		"unverifiable_address", "invalid_owner_info", "invalid_contact",
		"missing_documents", "ineligible_category", "other",
	}},
	StoreActionSuspend: {from: StoreStatusApproved, to: StoreStatusSuspended, reasons: []string{
		"customer_complaints", "missed_pickups", "frequent_cancellations", "expired_license",
		"policy_violation", "unpaid_balance", "vendor_request", "other",
	}},
	StoreActionReactivate: {from: StoreStatusSuspended, to: StoreStatusApproved},
}

// Transition returns the only status the action may start from and the
// status it leads to. ok is false for an unknown action.
func (a StoreAction) Transition() (from, to StoreStatus, ok bool) {
	t, ok := storeTransitions[a]
	return t.from, t.to, ok
}

// NeedsReason reports whether the action must carry a reason code and a
// message for the vendor (reject and suspend).
func (a StoreAction) NeedsReason() bool {
	return storeTransitions[a].reasons != nil
}

// ValidReason reports whether code is an accepted reason for the action.
func (a StoreAction) ValidReason(code string) bool {
	for _, r := range storeTransitions[a].reasons {
		if r == code {
			return true
		}
	}
	return false
}

// ReasonCodes returns the accepted reason codes of the action.
func (a StoreAction) ReasonCodes() []string {
	return append([]string(nil), storeTransitions[a].reasons...)
}
