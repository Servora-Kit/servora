package jwt

import (
	"context"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"

	"github.com/Servora-Kit/servora/security/authn"
)

// Server returns a Kratos server middleware that authenticates incoming
// requests with a single JWT engine. It is a power-user convenience wrapper
// for the most common single-engine direct mount:
//
//	jwt.Server(jwt.WithVerifier(v))
//
// Internally this is exactly equivalent to:
//
//	authn.Server(authn.Multi(authn.Named(jwt.Scheme, jwt.NewAuthenticator(opts...))))
//
// plus a pre-step that extracts the inbound Bearer token and stashes it into
// the jwt-private ctx channel ([WithToken]), so downstream middleware (e.g.
// [Client] for outbound propagation) and any business handler can read it
// via [TokenFrom].
//
// Business code that wires multiple engines (jwt + apikey, …) should NOT
// use this helper — call authn.Server + authn.Multi(authn.Named(...), …)
// directly so each engine contributes its own Named pair.
func Server(opts ...Option) middleware.Middleware {
	inner := authn.Server(
		authn.Multi(
			authn.Named(Scheme, NewAuthenticator(opts...)),
		),
	)
	return func(handler middleware.Handler) middleware.Handler {
		next := inner(handler)
		return func(ctx context.Context, req any) (any, error) {
			// Pre-extract Bearer + install jwt-private ctx channel BEFORE
			// dispatch, so:
			//   - the engine's transport-header fallback is bypassed (faster),
			//   - downstream Client() / business middleware can call
			//     TokenFrom(ctx) without re-parsing the header.
			//
			// authn.Server itself owns the chain short-circuit (skip dispatch
			// when ctx already carries a non-anonymous actor); we do NOT
			// short-circuit again here to keep wrapper semantics centralized.
			if tr, ok := transport.FromServerContext(ctx); ok {
				if raw := extractBearerToken(tr.RequestHeader().Get("Authorization")); raw != "" {
					ctx = WithToken(ctx, raw)
				}
			}
			return next(ctx, req)
		}
	}
}
