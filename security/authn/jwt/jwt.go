// Package jwt provides a generic Bearer JWT authentication skeleton for the
// engine-agnostic authn dispatcher. It owns its credential I/O end-to-end:
//
//   - [Server] is a power-user convenience: the canonical thin wrapper around
//     authn.Server + authn.Multi(authn.Named(jwt.Scheme, NewAuthenticator(...)))
//     for the single-engine direct mount case.
//   - [Client] is a Kratos client middleware that propagates the token from
//     the ctx into the outbound Authorization header.
//   - [WithToken] / [TokenFrom] are the jwt-package-private ctx channel; they
//     are the supported way to read or write the raw bearer token in flight.
//   - [NewAuthenticator] exposes the bare engine for power users who want to
//     compose their own wrapper (e.g. read the token from a non-HTTP carrier)
//     or who wire several engines into authn.Multi directly.
//
// Typical business usage:
//
//	import authjwt "github.com/Servora-Kit/servora/security/authn/jwt"
//
//	mw = append(mw, authjwt.Server(authjwt.WithVerifier(km.Verifier())))
//
// Multi-engine wiring (recommended for production):
//
//	mw = append(mw, authn.Server(
//	    authn.Multi(
//	        authn.Named(authjwt.Scheme, authjwt.NewAuthenticator(authjwt.WithVerifier(v))),
//	        authn.Named(apikey.Scheme, apikey.NewAuthenticator(...)),
//	    ),
//	    authn.WithRulesFuncs(examplev1.AuthnRules),
//	))
//
// The transport middleware package MUST NOT host equivalents of [WithToken]
// / [TokenFrom] / Bearer extraction — those are jwt-engine concerns, not
// framework-wide concerns.
package jwt

import (
	"context"
	"fmt"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/go-kratos/kratos/v2/transport"

	"github.com/Servora-Kit/servora/security/authn"
)

// Scheme is the canonical scheme string for this engine, paired with
// [NewAuthenticator] via [authn.Named]. The framework does not enumerate
// scheme constants — each engine sub-package owns its own string.
const Scheme = "jwt"

// Ensure *authenticator implements authn.Authenticator at compile time.
var _ authn.Authenticator = (*authenticator)(nil)

type authenticator struct {
	cfg *authenticatorConfig
}

// NewAuthenticator creates a JWT-based [authn.Authenticator].
//
// The returned authenticator's Authenticate(ctx) reads the raw bearer token
// from (in priority order):
//
//  1. The jwt-private ctx channel ([TokenFrom]) — typically populated by
//     [Server] before dispatch, or by upstream code via [WithToken].
//  2. The inbound Authorization header on the Kratos server transport — used
//     when the engine is wired into authn.Multi directly without [Server].
//
// If no token is found, ctx is returned unchanged with nil error
// (pass-through mode). If a token is found but no Verifier is configured,
// ctx is also returned unchanged (pass-through mode for local/test).
//
// Verifier failure → (ctx, err). ClaimsMapper failure → (ctx, err).
func NewAuthenticator(opts ...Option) authn.Authenticator {
	return newAuthenticator(opts...)
}

// newAuthenticator is the package-private constructor used by
// [NewAuthenticator] and [Server] to avoid duplicating Option processing;
// it returns the concrete *authenticator so jwt-package internals can read
// unexported fields. External callers should use [NewAuthenticator] which
// returns the [authn.Authenticator] interface.
func newAuthenticator(opts ...Option) *authenticator {
	cfg := &authenticatorConfig{
		claimsMapper: DefaultClaimsMapper(),
	}
	for _, opt := range opts {
		opt(cfg)
	}
	if cfg.claimsMapper == nil {
		cfg.claimsMapper = DefaultClaimsMapper()
	}
	return &authenticator{cfg: cfg}
}

// Authenticate reads the raw bearer token (ctx channel first, transport
// header fallback), verifies it, and returns an enriched context.
func (a *authenticator) Authenticate(ctx context.Context) (context.Context, error) {
	tokenString := tokenForAuth(ctx)
	if tokenString == "" {
		return ctx, nil
	}

	if a.cfg.verifier == nil {
		return ctx, nil
	}

	claims := gojwt.MapClaims{}
	if err := a.cfg.verifier.Verify(tokenString, claims); err != nil {
		return ctx, fmt.Errorf("jwt: verify token: %w", err)
	}

	enriched, err := a.cfg.claimsMapper(ctx, claims)
	if err != nil {
		return ctx, err
	}
	return authn.WithAuthType(enriched, "user"), nil
}

// tokenForAuth resolves the raw bearer token used for verification. The
// jwt-private ctx channel takes precedence so [Server] (which writes it
// after parsing the inbound header) wins over a stale or absent transport.
// When the engine is wired without [Server], the transport-header fallback
// makes single-engine and authn.Multi direct wiring still work.
func tokenForAuth(ctx context.Context) string {
	if raw, ok := TokenFrom(ctx); ok && raw != "" {
		return raw
	}
	tr, ok := transport.FromServerContext(ctx)
	if !ok {
		return ""
	}
	return extractBearerToken(tr.RequestHeader().Get("Authorization"))
}
