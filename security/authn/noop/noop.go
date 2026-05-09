// Package noop provides a no-op Authenticator that always returns an anonymous actor.
// Useful for testing or endpoints that do not require authentication.
package noop

import (
	"context"

	"github.com/Servora-Kit/servora/core/actor"
	"github.com/Servora-Kit/servora/security/authn"
)

// Ensure *Authenticator implements authn.Authenticator at compile time.
var _ authn.Authenticator = (*Authenticator)(nil)

// Authenticator is a no-op implementation that always returns an anonymous actor.
type Authenticator struct{}

// NewAuthenticator returns a no-op Authenticator that always produces an anonymous actor.
// The constructor name matches the jwt sub-package convention (NewAuthenticator) so that
// callers can swap engines without learning per-package idioms.
func NewAuthenticator() authn.Authenticator {
	return &Authenticator{}
}

// Authenticate always returns an anonymous actor with no error.
func (a *Authenticator) Authenticate(_ context.Context) (actor.Actor, error) {
	return actor.NewAnonymousActor(), nil
}
