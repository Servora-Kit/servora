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
func (a *Authorizer) Check(_ context.Context, _, _, _, _ string) (bool, error) {
	return true, nil
}

// BatchCheck returns all-allowed results matching the input length.
func (a *Authorizer) BatchCheck(_ context.Context, reqs []authz.CheckRequest) ([]authz.CheckResult, error) {
	out := make([]authz.CheckResult, len(reqs))
	for i := range reqs {
		out[i] = authz.CheckResult{Allowed: true}
	}
	return out, nil
}

// ListAllowed returns nil — the noop authorizer has no resource model.
// Callers needing real listing must use a real backend.
func (a *Authorizer) ListAllowed(_ context.Context, _, _, _ string) ([]string, error) {
	return nil, nil
}
