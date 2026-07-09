package authn

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrNoCredentials is returned by authentication engines when the request does
// not carry credentials for that engine. It is not an authentication failure by
// itself: composite dispatchers such as Multi use it to continue to the next
// allowed engine. Invalid credentials and backend/config failures must return
// any other non-nil error.
var ErrNoCredentials = errors.New("authn: no credentials")

// SchemeAttempt records one engine's outcome inside a `Multi` dispatch.
// Both fields are public so `WithErrorHandler` consumers can render
// per-scheme diagnostics (logging, metrics labels, custom RPC errors).
type SchemeAttempt struct {
	Scheme string
	Reason string
}

// SchemeAttemptsErr is the public interface satisfied by the package-private
// error type returned from `Multi.Authenticate` when every allowed engine
// failed. Business code obtains structured access via type assertion:
//
//	authn.WithErrorHandler(func(ctx context.Context, err error) error {
//	    if as, ok := err.(authn.SchemeAttemptsErr); ok {
//	        for _, a := range as.SchemeAttempts() {
//	            log.Errorw("authn attempt failed",
//	                "scheme", a.Scheme, "reason", a.Reason)
//	        }
//	    }
//	    return err
//	})
//
// `Server` itself uses this interface to render safe structured failure
// reasons.
type SchemeAttemptsErr interface {
	error
	SchemeAttempts() []SchemeAttempt
}

// schemeAttemptsErr is the package-private implementation of
// `SchemeAttemptsErr`. The slice is never mutated after construction.
type schemeAttemptsErr struct {
	attempts      []SchemeAttempt
	noCredentials bool
}

// Error renders the same "scheme: reason; scheme: reason" form that Server
// uses for failure responses. This keeps fallback callers and structured
// diagnostics consistent.
func (e *schemeAttemptsErr) Error() string {
	if e == nil || len(e.attempts) == 0 {
		return "authn: all schemes failed"
	}
	parts := make([]string, 0, len(e.attempts))
	for _, a := range e.attempts {
		parts = append(parts, fmt.Sprintf("%s: %s", a.Scheme, a.Reason))
	}
	return strings.Join(parts, "; ")
}

func (e *schemeAttemptsErr) Is(target error) bool {
	return e != nil && e.noCredentials && target == ErrNoCredentials
}

// SchemeAttempts returns a copy of the per-scheme attempt slice. A copy is
// returned so business handlers cannot mutate the underlying error state.
func (e *schemeAttemptsErr) SchemeAttempts() []SchemeAttempt {
	if e == nil || len(e.attempts) == 0 {
		return nil
	}
	out := make([]SchemeAttempt, len(e.attempts))
	copy(out, e.attempts)
	return out
}

// errSchemesEmpty is returned by `Multi.Authenticate` when the
// `allowedSchemes` set installed by `Server` does not intersect any of
// the configured engines (e.g. proto declares `schemes=["mtls"]` but
// business only wired a jwt engine). The caller (typically `Server`)
// surfaces this as a normal authn failure.
var errSchemesEmpty = errors.New("authn: allowed schemes empty for this method (no engine matched)")

// NamedAuthenticator is an opaque scheme + Authenticator pair produced by
// `Named`. Fields are package-private; consumers can only construct via
// `Named` and consume via `Multi`.
type NamedAuthenticator struct {
	scheme string
	inner  Authenticator
}

// Named pairs a scheme string with an `Authenticator` for `Multi`.
//
// The scheme string is opaque to the framework — any value is accepted.
// Engine sub-packages typically expose a `Scheme` constant
// (e.g. `jwt.Scheme = "jwt"`) so business code can write
// `authn.Named(jwt.Scheme, jwt.NewAuthenticator(...))`.
func Named(scheme string, a Authenticator) NamedAuthenticator {
	if scheme == "" {
		panic("authn: Named scheme is required")
	}
	if a == nil {
		panic("authn: Named authenticator is required")
	}
	return NamedAuthenticator{scheme: scheme, inner: a}
}

// multi is the package-private implementation behind `Multi`. The exported
// constructor returns the `Authenticator` interface so callers can compose
// it transparently with single-engine implementations.
type multi struct {
	engines []NamedAuthenticator
}

// Multi composes multiple `NamedAuthenticator` engines into a single
// `Authenticator`. First-success-wins: engines are tried in injection
// order (NOT in `allowedSchemes` order — business decides the precedence
// at wiring time).
//
// If the `allowedSchemes` set installed by `Server` is non-nil, engines whose
// scheme is absent from the set are skipped silently. If nil, every engine
// participates.
//
// Wrapping a single engine via `Multi(Named(...))` is a supported and
// recommended pattern: it gives the dispatcher the scheme name without
// requiring a separate `WithMethod` option, and makes future expansion
// to more engines a one-line change.
func Multi(named ...NamedAuthenticator) Authenticator {
	if len(named) == 0 {
		panic("authn: Multi requires at least one authenticator")
	}
	seen := make(map[string]struct{}, len(named))
	for _, n := range named {
		if n.scheme == "" {
			panic("authn: Named scheme is required")
		}
		if n.inner == nil {
			panic("authn: Named authenticator is required")
		}
		if _, ok := seen[n.scheme]; ok {
			panic(fmt.Sprintf("authn: duplicate scheme %q", n.scheme))
		}
		seen[n.scheme] = struct{}{}
	}
	return &multi{engines: named}
}

// Authenticate iterates engines in injection order; the first to return
// (enrichedCtx, nil) wins. See `Multi` doc for filter semantics.
func (m *multi) Authenticate(ctx context.Context) (context.Context, error) {
	allowed := allowedSchemesFrom(ctx)

	var attempts []SchemeAttempt
	participated := 0
	for _, named := range m.engines {
		if allowed != nil {
			if _, ok := allowed[named.scheme]; !ok {
				continue
			}
		}
		participated++
		enrichedCtx, err := named.inner.Authenticate(ctx)
		if err == nil {
			return enrichedCtx, nil
		}
		attempts = append(attempts, SchemeAttempt{
			Scheme: named.scheme,
			Reason: err.Error(),
		})
		if !errors.Is(err, ErrNoCredentials) {
			return ctx, &schemeAttemptsErr{attempts: attempts}
		}
	}

	if participated == 0 {
		return ctx, errSchemesEmpty
	}
	return ctx, &schemeAttemptsErr{attempts: attempts, noCredentials: true}
}
