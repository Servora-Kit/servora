// Package noop provides a no-op Authorizer that always permits all requests.
// Useful for testing or services that do not require authorization enforcement.
package noop

import (
	"context"

	"github.com/Servora-Kit/servora/security/authz"
)

// Ensure *Authorizer implements authz.Authorizer at compile time.
var _ authz.Authorizer = (*Authorizer)(nil)

// Authorizer is a no-op implementation that always returns allowed=true.
type Authorizer struct{}

// New returns a NoopAuthorizer that always permits requests.
func New() authz.Authorizer { return &Authorizer{} }

// Check always returns (true, nil).
func (a *Authorizer) Check(_ context.Context, _ authz.CheckRequest) (bool, error) {
	return true, nil
}
