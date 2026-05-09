package jwt

import (
	"context"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
)

// Client returns a Kratos client middleware that propagates the jwt token
// previously stored in the ctx (by [Server] on the inbound side, or by an
// explicit [WithToken] call) into the outbound Authorization header as
// `Bearer <token>`.
//
// If no token is present in the ctx or no client transport is attached, the
// middleware passes through without modification — never errors.
//
// Business callers must opt in explicitly: the framework's default client
// chain does NOT include this middleware, because not every outbound call
// wants to forward an inbound credential (cross-realm calls, third-party
// integrations, etc.). See design.md Decision 5.
//
// Typical usage:
//
//	conn, err := grpc.Dial(grpc.WithMiddleware(jwt.Client()))
func Client() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			tok, ok := TokenFromContext(ctx)
			if !ok || tok == "" {
				return handler(ctx, req)
			}
			tr, ok := transport.FromClientContext(ctx)
			if !ok {
				return handler(ctx, req)
			}
			tr.RequestHeader().Set("Authorization", "Bearer "+tok)
			return handler(ctx, req)
		}
	}
}
