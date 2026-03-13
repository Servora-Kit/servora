package biz

import "fmt"

var (
	validOrganizationRoles = map[string]bool{"owner": true, "admin": true, "member": true, "viewer": true}
	validProjectRoles      = map[string]bool{"admin": true, "member": true, "viewer": true}
)

func ValidateOrganizationRole(role string) error {
	if !validOrganizationRoles[role] {
		return fmt.Errorf("invalid organization role %q; allowed: owner, admin, member, viewer", role)
	}
	return nil
}

func ValidateProjectRole(role string) error {
	if !validProjectRoles[role] {
		return fmt.Errorf("invalid project role %q; allowed: admin, member, viewer", role)
	}
	return nil
}
