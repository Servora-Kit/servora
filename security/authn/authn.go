// Package authn provides a generic Kratos middleware for JWT-based authentication.
// It is engine-agnostic: any Authenticator implementation can be injected.
//
// Example usage:
//
//	import (
//	    "github.com/Servora-Kit/servora/security/authn"
//	    "github.com/Servora-Kit/servora/security/authn/jwt"
//	)
//
//	mw = append(mw, authn.Server(
//	    jwt.NewAuthenticator(jwt.WithVerifier(km.Verifier())),
//	))
package authn

import (
	"context"
	"strings"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"

	"github.com/Servora-Kit/servora/core/actor"
	svrmw "github.com/Servora-Kit/servora/transport/server/middleware"
)

// Authenticator is the interface for authenticating incoming requests.
// Implementations receive the full request context (which may include
// the raw token stored by Server) and return an actor.Actor.
type Authenticator interface {
	Authenticate(ctx context.Context) (actor.Actor, error)
}

// AuthnDetail captures the outcome of one authentication attempt for the
// observer callback. Note: this is distinct from obs/audit.AuthnDetail
// (which is the audit event payload). The mapping between them lives in
// obs/audit/observers.go.
//
// Coverage: JWT engine (current) and future mTLS app-layer SAN/XFCC
// failures. Does NOT cover TLS handshake errors — those should be
// captured by transport-level metrics + logs (see TODO P1-3 mTLS plan).
type AuthnDetail struct {
	Method  string      // "jwt" / "mtls" / ...
	Subject actor.Actor // resolved actor on success; actor.NewAnonymousActor() on failure
	Allowed bool        // true on success (incl. anonymous success); false on failure
	Err     error       // nil on success; original error on failure
}

// Option configures the Server middleware.
type Option func(*serverConfig)

type serverConfig struct {
	errorHandler func(ctx context.Context, err error) error
	observer     func(context.Context, AuthnDetail)
}

// WithErrorHandler sets a custom error handler invoked when authentication fails.
func WithErrorHandler(h func(ctx context.Context, err error) error) Option {
	return func(c *serverConfig) { c.errorHandler = h }
}

// WithObserver installs a callback invoked after every Authenticate call
// (success or failure). Pair it with `recorder.AuthnObserver()` from
// obs/audit to bridge results into the audit pipeline:
//
//	authn.Server(authenticator,
//	    authn.WithObserver(recorder.AuthnObserver()),
//	)
//
// observer == nil leaves the middleware unaffected (no-op).
func WithObserver(fn func(ctx context.Context, d AuthnDetail)) Option {
	return func(c *serverConfig) { c.observer = fn }
}

// Server returns a Kratos middleware that authenticates requests using the provided Authenticator.
// It extracts the Bearer token from the Authorization header, stores it in context via
// svrmw.NewTokenContext, then delegates to the Authenticator to produce an actor.Actor.
//
// Behavior:
//   - No transport in context → anonymous actor injected, handler called
//   - No Authorization header → anonymous actor injected (authenticator may override)
//   - Authenticator error + no error handler → error returned
//   - Authenticator error + error handler → handler's return value used
func Server(authenticator Authenticator, opts ...Option) middleware.Middleware {
	cfg := &serverConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			tr, ok := transport.FromServerContext(ctx)
			if !ok {
				// No-transport early-return is treated as "anonymous success":
				// no authenticator runs, so we surface an anonymous-allowed
				// detail to the observer for symmetry with the JWT engine
				// returning an anonymous actor on missing-header.
				if cfg.observer != nil {
					cfg.observer(ctx, AuthnDetail{
						Method:  "jwt",
						Subject: actor.NewAnonymousActor(),
						Allowed: true,
						Err:     nil,
					})
				}
				ctx = actor.NewContext(ctx, actor.NewAnonymousActor())
				return handler(ctx, req)
			}

			// Extract raw token and store for downstream consumers (e.g. opaque pass-through).
			if tokenString := ExtractBearerToken(tr.RequestHeader().Get("Authorization")); tokenString != "" {
				ctx = svrmw.NewTokenContext(ctx, tokenString)
			}

			a, err := authenticator.Authenticate(ctx)
			if err != nil {
				if cfg.observer != nil {
					cfg.observer(ctx, AuthnDetail{
						Method:  "jwt",
						Subject: actor.NewAnonymousActor(),
						Allowed: false,
						Err:     err,
					})
				}
				if cfg.errorHandler != nil {
					return nil, cfg.errorHandler(ctx, err)
				}
				return nil, err
			}

			if cfg.observer != nil {
				cfg.observer(ctx, AuthnDetail{
					Method:  "jwt",
					Subject: a,
					Allowed: true,
					Err:     nil,
				})
			}
			ctx = actor.NewContext(ctx, a)
			return handler(ctx, req)
		}
	}
}

// ExtractBearerToken parses the Bearer token from an Authorization header value.
// Returns empty string if the header is absent or malformed.
func ExtractBearerToken(header string) string {
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return parts[1]
}
