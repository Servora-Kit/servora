// Package jwt provides a generic Bearer JWT authentication skeleton for the
// engine-agnostic authn dispatcher. It owns its credential I/O end-to-end:
//
//   - [Client] is a Kratos client middleware that propagates the token from
//     the ctx into the outbound Authorization header.
//   - [WithToken] / [TokenFrom] are the jwt-package-private ctx channel; they
//     are the supported way to read or write the raw bearer token in flight.
//   - [NewAuthenticator] exposes the bare engine wired through authn.Named and
//     authn.Multi.
//
// Typical business usage:
//
//	import authjwt "github.com/Servora-Kit/servora/security/authn/jwt"
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

import "github.com/Servora-Kit/servora/security/authn"

// Scheme is the canonical scheme string for this engine, paired with
// [NewAuthenticator] via [authn.Named]. The framework does not enumerate
// scheme constants — each engine sub-package owns its own string.
const Scheme = "jwt"

// Ensure *authenticator implements authn.Authenticator at compile time.
var _ authn.Authenticator = (*authenticator)(nil)
