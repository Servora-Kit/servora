// Package noop provides a no-op Authenticator that passes through without enrichment.
// Useful for testing or endpoints that do not require authentication.
package noop

import (
	"context"

	"github.com/Servora-Kit/servora/security/authn"
)

var _ authn.Authenticator = (*Authenticator)(nil)

type Authenticator struct{}

func NewAuthenticator() authn.Authenticator {
	return &Authenticator{}
}

func (a *Authenticator) Authenticate(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
