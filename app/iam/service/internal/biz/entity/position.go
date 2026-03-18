package entity

import "time"

type Position struct {
	ID             string
	TenantID       string
	OrganizationID *string
	Code           string
	Name           string
	Description    *string
	Sort           int
	Status         string // ACTIVE | DISABLED
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
