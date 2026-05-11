// Package batch defines the optional BatchAuthorizer sub-interface for
// authorization backends that support multi-check in a single round-trip.
package batch

import (
	"context"

	"github.com/Servora-Kit/servora/security/authz"
)

// BatchAuthorizer extends Authorizer with a BatchCheck method that runs
// multiple authorization checks in one round-trip. Output order matches input.
//
// Implementations may internally chunk if the backend has per-call limits
// (e.g. OpenFGA caps at 50 per request).
type BatchAuthorizer interface {
	authz.Authorizer
	BatchCheck(ctx context.Context, reqs []authz.CheckRequest) ([]bool, error)
}
