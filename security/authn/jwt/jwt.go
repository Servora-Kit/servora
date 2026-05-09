// Package jwt provides a self-contained JWT authentication wrapper for the
// engine-agnostic authn dispatcher. It owns its credential I/O end-to-end:
//
//   - [Server] is a Kratos server middleware that extracts the Bearer token
//     from the inbound Authorization header, stores it into the jwt-private
//     ctx channel, and delegates to authn.Server with method "jwt".
//   - [Client] is a Kratos client middleware that propagates the token from
//     the ctx into the outbound Authorization header.
//   - [WithToken] / [TokenFromContext] are the jwt-package-private ctx
//     channel; they are the ONLY supported way to read or write the raw
//     bearer token in flight.
//   - [NewAuthenticator] exposes the bare engine for power users who want to
//     compose their own wrapper (e.g. read the token from a non-HTTP carrier).
//
// Typical business usage:
//
//	import authjwt "github.com/Servora-Kit/servora/security/authn/jwt"
//
//	mw = append(mw, authjwt.Server(authjwt.WithVerifier(km.Verifier())))
//
// The general transport middleware package MUST NOT host equivalents of
// [WithToken] / [TokenFromContext] / Bearer extraction — those are jwt-engine
// concerns, not framework-wide concerns. See design.md Decisions 4 / 5 / 6.
package jwt

import (
	"context"

	gojwt "github.com/golang-jwt/jwt/v5"

	"github.com/Servora-Kit/servora/core/actor"
	"github.com/Servora-Kit/servora/security/authn"
)

// Ensure *authenticator implements authn.Authenticator at compile time.
var _ authn.Authenticator = (*authenticator)(nil)

type authenticator struct {
	cfg *authenticatorConfig
}

// NewAuthenticator creates a JWT-based [authn.Authenticator] without the
// surrounding wrapper. Power users who need to compose their own wrapper
// (e.g. read the token from a non-HTTP carrier) can pair this with
// authn.Server directly:
//
//	auth := jwt.NewAuthenticator(jwt.WithVerifier(v))
//	mw := authn.Server(auth, authn.WithMethod("jwt"))
//
// Most callers should use [Server] instead, which assembles the complete
// wrapper (Bearer extraction + dispatcher + chain short-circuit).
//
// The token is read from the jwt-private ctx channel via [TokenFromContext].
// If no token is present, or no Verifier is configured, an anonymous actor
// is returned with nil error.
func NewAuthenticator(opts ...Option) authn.Authenticator {
	return newAuthenticator(opts...)
}

// newAuthenticator is the package-private constructor used by
// [NewAuthenticator] and [Server] / serverWith to avoid duplicating Option
// processing; it returns the concrete *authenticator so jwt-package internals
// can read unexported fields. External callers should use [NewAuthenticator]
// which returns the [authn.Authenticator] interface.
func newAuthenticator(opts ...Option) *authenticator {
	cfg := &authenticatorConfig{
		claimsMapper: DefaultClaimsMapper(),
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return &authenticator{cfg: cfg}
}

// Authenticate reads the raw token from the jwt-private ctx channel
// ([TokenFromContext]), verifies it, and returns an actor.Actor.
func (a *authenticator) Authenticate(ctx context.Context) (actor.Actor, error) {
	tokenString, ok := TokenFromContext(ctx)
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
