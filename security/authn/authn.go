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
//
// The middleware writes a *auditpb.AuthnDetail to ctx via
// audit.WithAuthnResult; emission is the responsibility of the transport-tail
// audit.Collector middleware. The authn package therefore has zero coupling
// to the audit emission pipeline (only to the neutral auditpb schema package).
package authn

import (
	"context"
	"strings"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"

	auditpb "github.com/Servora-Kit/servora/api/gen/go/servora/audit/v1"
	"github.com/Servora-Kit/servora/core/actor"
	"github.com/Servora-Kit/servora/obs/audit"
	svrmw "github.com/Servora-Kit/servora/transport/server/middleware"
)

// Authenticator is the interface for authenticating incoming requests.
//
// CONTRACT: this interface intentionally contains only TWO kinds of members:
//  1. Authentication behavior (Authenticate)
//  2. Engine immutable metadata (Method — self-description)
//
// Hooks/callbacks (e.g. OnSuccess), injection (logger/tracer), infra
// probes (Health) are explicitly NOT permitted here. Those concerns
// belong to callers, containers, or optional sibling interfaces.
//
// This rule prevents interface bloat as new engines (mTLS, etc.) are
// added: each engine is described by Method(), and orchestration is
// the middleware's responsibility.
type Authenticator interface {
	Authenticate(ctx context.Context) (actor.Actor, error)
	Method() string
}

// Option configures the Server middleware.
type Option func(*serverConfig)

type serverConfig struct {
	errorHandler func(ctx context.Context, err error) error
}

// WithErrorHandler sets a custom error handler invoked when authentication fails.
func WithErrorHandler(h func(ctx context.Context, err error) error) Option {
	return func(c *serverConfig) { c.errorHandler = h }
}

// Server returns a Kratos middleware that authenticates requests using the provided Authenticator.
// It extracts the Bearer token from the Authorization header, stores it in context via
// svrmw.NewTokenContext, then delegates to the Authenticator to produce an actor.Actor.
//
// Behavior:
//   - No transport in context → anonymous actor injected, anonymous-success
//     AuthnDetail written, handler called.
//   - No Authorization header → anonymous actor injected (authenticator may
//     override); detail reflects authenticator outcome.
//   - Authenticator success → user-actor + Success=true detail in ctx.
//   - Authenticator error + no error handler → failure detail written
//     BEFORE returning the error (so a downstream Collector can still emit).
//   - Authenticator error + error handler → handler's return value used,
//     failure detail still written first.
func Server(authenticator Authenticator, opts ...Option) middleware.Middleware {
	cfg := &serverConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	// Cache Method() once at middleware construction. Engine metadata is
	// immutable per the interface contract, so per-request dispatch is
	// wasted work.
	method := authenticator.Method()

	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			tr, ok := transport.FromServerContext(ctx)
			if !ok {
				// No-transport early-return is treated as "anonymous success":
				// no authenticator runs, but we still surface a detail to
				// keep the audit pipeline symmetric with the in-engine
				// missing-header path.
				ctx = audit.WithAuthnResult(ctx, &auditpb.AuthnDetail{
					Method:        method,
					Success:       true,
					FailureReason: "",
				})
				ctx = actor.NewContext(ctx, actor.NewAnonymousActor())
				return handler(ctx, req)
			}

			// Extract raw token and store for downstream consumers (e.g. opaque pass-through).
			if tokenString := ExtractBearerToken(tr.RequestHeader().Get("Authorization")); tokenString != "" {
				ctx = svrmw.NewTokenContext(ctx, tokenString)
			}

			a, err := authenticator.Authenticate(ctx)
			if err != nil {
				ctx = audit.WithAuthnResult(ctx, &auditpb.AuthnDetail{
					Method:        method,
					Success:       false,
					FailureReason: err.Error(),
				})
				if cfg.errorHandler != nil {
					return nil, cfg.errorHandler(ctx, err)
				}
				return nil, err
			}

			ctx = audit.WithAuthnResult(ctx, &auditpb.AuthnDetail{
				Method:        method,
				Success:       true,
				FailureReason: "",
			})
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
