// Package enum holds closed sets of domain values.
package enum

type Role string

const (
	RoleCustomer Role = "customer"
	RoleVendor   Role = "vendor"
	RoleAdmin    Role = "admin"
)

func (r Role) Valid() bool {
	switch r {
	case RoleCustomer, RoleVendor, RoleAdmin:
		return true
	}
	return false
}
