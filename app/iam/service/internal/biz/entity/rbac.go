package entity

import "time"

type RbacRole struct {
	ID            string
	Code          string
	Name          string
	Description   string
	Type          string // BUILTIN or CUSTOM
	IsProtected   bool
	Status        string // ACTIVE or DISABLED
	TenantID      *string
	PermissionIDs []string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type RbacPermission struct {
	ID          string
	Code        string
	Name        string
	Description string
	GroupID     *string
	GroupName   string
	Status      string
	CreatedAt   time.Time
	APIs        []RbacPermissionAPI
}

type RbacPermissionAPI struct {
	Method string
	Path   string
}

type RbacPermissionGroup struct {
	ID       string
	Name     string
	Module   string
	ParentID *string
	Sort     int
	Children []*RbacPermissionGroup
}

type RbacMenu struct {
	ID        string
	Type      string // CATALOG, MENU, BUTTON
	Name      string
	Path      string
	Component string
	Redirect  string
	Meta      string
	ParentID  *string
	Sort      int
	Status    string
	Children  []*RbacMenu
}
