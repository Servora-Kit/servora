// Package jwt provides a JWT-based Authenticator implementation for pkg/authn.
// Use NewAuthenticator to create an instance and pass it to authn.Server().
package jwt

import (
	"context"

	gojwt "github.com/golang-jwt/jwt/v5"

	"github.com/Servora-Kit/servora/core/actor"
	"github.com/Servora-Kit/servora/security/authn"
	svrmw "github.com/Servora-Kit/servora/transport/server/middleware"
)

// Ensure *authenticator implements authn.Authenticator at compile time.
var _ authn.Authenticator = (*authenticator)(nil)

type authenticator struct {
	cfg *authenticatorConfig
}

// NewAuthenticator creates a JWT-based Authenticator.
// The token is read from context (stored by authn.Server via svrmw.NewTokenContext).
// If no token is present, or no verifier is configured, an anonymous actor is returned.
func NewAuthenticator(opts ...Option) authn.Authenticator {
	cfg := &authenticatorConfig{
		claimsMapper: DefaultClaimsMapper(),
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return &authenticator{cfg: cfg}
}

// Authenticate reads the raw token from context, verifies it, and returns an actor.Actor.
func (a *authenticator) Authenticate(ctx context.Context) (actor.Actor, error) {
	tokenString, ok := svrmw.TokenFromContext(ctx)
	if !ok || tokenString == "" {
		return actor.NewAnonymousActor(), nil
	}

	if a.cfg.verifier == nil {
		return actor.NewAnonymousActor(), nil
	}

	claims := gojwt.MapClaims{}
	if err := a.cfg.verifier.Verify(tokenString, claims); err != nil {
		return nil, err
	}

	return a.cfg.claimsMapper(claims)
}
