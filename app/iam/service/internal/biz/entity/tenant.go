package entity

import "time"

type Tenant struct {
	ID          string
	OwnerUserID string
	Slug        string
	Name        string
	DisplayName string
	Domain      string
	Kind        string // "business" | "personal"
	Status      string // "active" | "disabled"
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
