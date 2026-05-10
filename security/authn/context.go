package authn

import "context"

// This file holds the two package-private ctx channels used by the
// `Server` dispatcher and the `Multi` decorator to communicate without
// extending the public `Authenticator` interface.
//
//   - allowedSchemes — Server writes (from Rules.MethodSchemes), Multi
//     reads (to filter engines).
//   - successfulScheme (mutable holder) — Server installs an empty
//     holder, Multi writes the winning scheme on success, Server reads
//     after the engine returns.
//
// Both keys and accessors are intentionally unexported: external packages
// (including engine sub-packages such as `security/authn/jwt`) MUST NOT
// participate in these channels directly.

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

// --- successfulScheme mutable holder channel ---------------------------

type successfulSchemeKey struct{}

// schemeHolder is a mutable holder for the scheme string of whichever
// engine ultimately succeeded under `Multi`. Same pattern as the audit
// detailHolder (P0-5): the parent attaches an empty holder, the child
// mutates the field in place, the parent reads it after dispatch — all
// without extending the child's interface signature.
type schemeHolder struct {
	scheme string
}

// installSchemeHolder attaches a fresh empty holder. `Server` installs
// before calling `Authenticate`; the holder lives for the lifetime of
// that single dispatch call.
func installSchemeHolder(ctx context.Context) context.Context {
	return context.WithValue(ctx, successfulSchemeKey{}, &schemeHolder{})
}

// schemeHolderFrom retrieves the holder attached by `installSchemeHolder`,
// or nil if absent (e.g. a single-engine business that does not flow
// through `Server`'s install path — discouraged but tolerated).
func schemeHolderFrom(ctx context.Context) *schemeHolder {
	h, _ := ctx.Value(successfulSchemeKey{}).(*schemeHolder)
	return h
}

// set records the winning scheme. Called by `Multi` on first success.
func (h *schemeHolder) set(scheme string) { h.scheme = scheme }

// get returns the recorded scheme, or "" if no engine wrote (single
// engine that bypassed `Multi`, or pre-dispatch error).
func (h *schemeHolder) get() string { return h.scheme }
