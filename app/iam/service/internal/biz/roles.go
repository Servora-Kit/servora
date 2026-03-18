package biz

import "fmt"

// Role represents an IAM role within an organization.
// Tenant level only has the "owner" concept stored as owner_user_id field on the tenant,
// not as a role enum. Organization level uses admin > member (two tiers).
type Role string

const (
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

// CanManageMembers returns true if the role is admin.
func (r Role) CanManageMembers() bool { return r == RoleAdmin }

// Level returns the numeric level of the role for hierarchy comparisons.
// Higher value means higher privilege.
func (r Role) Level() int {
	switch r {
	case RoleAdmin:
		return 1
	case RoleMember:
		return 0
	default:
		return -1
	}
}

// String returns the string representation of the role.
func (r Role) String() string { return string(r) }

// ValidateOrganizationRole validates that a role string is valid for organization membership.
func ValidateOrganizationRole(role string) error {
	r := Role(role)
	if r != RoleAdmin && r != RoleMember {
		return fmt.Errorf("invalid organization role %q; allowed: admin, member", role)
	}
	return nil
}
