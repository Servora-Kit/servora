package entity

import "time"

type DictType struct {
	ID          string
	Code        string
	Name        string
	Description *string
	Status      string // ACTIVE | DISABLED
	Sort        int
	Items       []*DictItem
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type DictItem struct {
	ID         string
	DictTypeID string
	Label      string
	Value      string
	ColorTag   *string
	Sort       int
	Status     string // ACTIVE | DISABLED
	IsDefault  bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
