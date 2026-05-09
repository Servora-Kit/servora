package jwt

import (
	"context"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"

	"github.com/Servora-Kit/servora/core/actor"
	"github.com/Servora-Kit/servora/security/authn"
)

// methodName is the engine identifier written into auditpb.AuthnDetail.Method
// on every dispatch through [Server]. Package-private on purpose: the
// framework does NOT enumerate scheme constants — each engine sub-package
// owns its own string. See design.md Decision 8.
const methodName = "jwt"

// Server returns a Kratos server middleware that authenticates incoming
// requests with the JWT engine. It wraps [authn.Server], adding jwt-specific
// credential I/O before delegation:
//
//  1. Chain short-circuit: if the ctx already carries a non-anonymous actor
//     (a previous engine in a multi-mechanism chain authenticated successfully),
//     pass through without invoking the engine — zero-cost hook used by the
//     P0-4b "auth chain" composition path. The pass-through branch does NOT
//     write an AuthnDetail (the prior engine already did) and does NOT touch
//     the actor.
//  2. Extract Bearer token from the inbound Authorization header and stash it
//     into the jwt-private ctx channel via [WithToken].
//  3. Delegate to authn.Server(engine, authn.WithMethod("jwt")) which calls
//     Authenticate, writes the AuthnDetail, and injects the resulting actor.
//
// Use this in business code instead of calling authn.Server directly:
//
//	mw = append(mw, jwt.Server(jwt.WithVerifier(km.Verifier())))
func Server(opts ...Option) middleware.Middleware {
	return serverWith(newAuthenticator(opts...))
}

// serverWith builds the wrapper middleware around an arbitrary
// authn.Authenticator. Package-private; used by [Server] and by tests that
// need to inject a stub Authenticator without going through the public
// Option chain (which requires a real Verifier dep).
//
// The composition exactly mirrors what Server() produces — keep them in
// lockstep when changing wrapper semantics.
func serverWith(auth authn.Authenticator) middleware.Middleware {
	// Build the authn dispatcher once at construction time; the resulting
	// middleware is reused per request.
	innerMW := authn.Server(auth, authn.WithMethod(methodName))
	return func(handler middleware.Handler) middleware.Handler {
		// Compose the dispatcher with the user handler exactly once. Without
		// this, every request would rebuild the inner closure — wasteful and
		// surprising.
		wrapped := innerMW(handler)
		return func(ctx context.Context, req any) (any, error) {
			// 1) Chain short-circuit: a non-anonymous actor injected by an
			// earlier wrapper in a multi-mechanism chain (P0-4b留口) means
			// authentication already succeeded; pass through without touching
			// the engine or the audit channel.
			if a, ok := actor.FromContext(ctx); ok && a.Type() != actor.TypeAnonymous {
				return handler(ctx, req)
			}
			// 2) Extract jwt-shaped credential from the inbound request and
			// stash it into the jwt-private ctx channel; the engine reads it
			// from there in Authenticate.
			if tr, ok := transport.FromServerContext(ctx); ok {
				if raw := extractBearerToken(tr.RequestHeader().Get("Authorization")); raw != "" {
					ctx = WithToken(ctx, raw)
				}
			}
			// 3) Delegate to the dispatcher (writes AuthnDetail, injects actor).
			return wrapped(ctx, req)
		}
	}
}
