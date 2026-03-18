package biz

import (
	"testing"
)

func TestRole_Level(t *testing.T) {
	tests := []struct {
		role  Role
		level int
	}{
		{RoleAdmin, 1},
		{RoleMember, 0},
		{Role("unknown"), -1},
	}
	for _, tt := range tests {
		if got := tt.role.Level(); got != tt.level {
			t.Errorf("Role(%q).Level() = %d, want %d", tt.role, got, tt.level)
		}
	}
}

func TestRole_CanManageMembers(t *testing.T) {
	if !RoleAdmin.CanManageMembers() {
		t.Error("RoleAdmin.CanManageMembers() = false, want true")
	}
	if RoleMember.CanManageMembers() {
		t.Error("RoleMember.CanManageMembers() = true, want false")
	}
}

func TestValidateOrganizationRole(t *testing.T) {
	if err := ValidateOrganizationRole("admin"); err != nil {
		t.Errorf("ValidateOrganizationRole(admin) = %v, want nil", err)
	}
	if err := ValidateOrganizationRole("member"); err != nil {
		t.Errorf("ValidateOrganizationRole(member) = %v, want nil", err)
	}
	if err := ValidateOrganizationRole("viewer"); err == nil {
		t.Error("ValidateOrganizationRole(viewer) = nil, want error (viewer removed)")
	}
	if err := ValidateOrganizationRole("owner"); err == nil {
		t.Error("ValidateOrganizationRole(owner) = nil, want error (owner not a valid org role)")
	}
	if err := ValidateOrganizationRole("invalid"); err == nil {
		t.Error("ValidateOrganizationRole(invalid) = nil, want error")
	}
}
