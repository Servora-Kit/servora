package jwt

import "context"

// tokenKey is the package-private ctx key under which the raw bearer token
// flows from [Server] (inbound extraction) to the engine [Authenticate], and
// from arbitrary callers to [Client] (outbound propagation).
type tokenKey struct{}

// WithToken stores the raw bearer token into a jwt-package-private ctx
// channel. It is invoked by [Server] after parsing the inbound Authorization
// header, and may also be called directly by upstream code that obtained a
// token via some other path (e.g., re-injection during a retry, or a
// non-HTTP carrier feeding a custom wrapper around [NewAuthenticator]).
//
// The channel is intentionally jwt-private. The general transport middleware
// package MUST NOT host equivalent helpers — credential carrier shape (the
// Bearer token format) is jwt-engine concern, not framework-wide concern.
func WithToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, tokenKey{}, token)
}

// TokenFrom reads the raw bearer token previously stored by [WithToken].
// It is used by [Client] for outbound propagation, and may be used by
// business middleware that wants to observe the inbound token. Returns
// ("", false) if no token is present.
func TokenFrom(ctx context.Context) (string, bool) {
	t, ok := ctx.Value(tokenKey{}).(string)
	return t, ok
}
