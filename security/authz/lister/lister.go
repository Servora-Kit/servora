// Package lister defines the optional Lister sub-interface for authorization
// backends that can enumerate resources a subject is allowed to access.
package lister

import "context"

// Lister returns IDs of resources (of resourceType) the subject has the given
// action on. The returned strings are bare IDs without "type:" prefix.
// Useful for "list" endpoints — caller fetches by `WHERE id IN (...)`.
type Lister interface {
	ListAllowed(ctx context.Context, subject, action, resourceType string) ([]string, error)
}
