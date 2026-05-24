package authn

import "context"

// This file holds the package-private ctx channel used by the `Server`
// dispatcher and the `Multi` decorator to communicate without extending the
// public `Authenticator` interface.
//
// allowedSchemes is written by Server from Rules.MethodSchemes and read by
// Multi to filter engines. Keys and accessors are intentionally unexported:
// external packages, including engine sub-packages, MUST NOT participate in
// this channel directly.

// --- allowedSchemes ctx channel ----------------------------------------

type allowedSchemesKey struct{}

// withAllowedSchemes attaches the allowed scheme set computed by `Server`
// from `Rules.MethodSchemes`. A nil `allowed` is permitted and signals
// "no restriction" (`Multi` then tries every engine).
func withAllowedSchemes(ctx context.Context, allowed map[string]struct{}) context.Context {
	return context.WithValue(ctx, allowedSchemesKey{}, allowed)
}

// allowedSchemesFrom returns the allowed-schemes set previously installed
// by `withAllowedSchemes`, or nil if absent.
func allowedSchemesFrom(ctx context.Context) map[string]struct{} {
	v, _ := ctx.Value(allowedSchemesKey{}).(map[string]struct{})
	return v
}
