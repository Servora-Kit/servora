package entity

import "time"

type User struct {
	ID              string
	Name            string
	Email           string
	Password        string
	Role            string
	EmailVerified   bool
	EmailVerifiedAt *time.Time
	// OrganizationIDs holds the IDs of organizations this user belongs to
	// within the current tenant scope. Populated on ListUsers.
	OrganizationIDs []string
}
