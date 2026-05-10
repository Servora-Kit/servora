package authn

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Servora-Kit/servora/core/actor"
)

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
// `Server` itself uses this interface to detect Multi-failures and tag
// `AuthnDetail.Method = "multi"` with a serialized aggregate FailureReason.
type SchemeAttemptsErr interface {
	error
	SchemeAttempts() []SchemeAttempt
}

// schemeAttemptsErr is the package-private implementation of
// `SchemeAttemptsErr`. The slice is never mutated after construction.
type schemeAttemptsErr struct {
	attempts []SchemeAttempt
}

// Error renders the same "scheme: reason; scheme: reason" form that
// `Server` uses for `AuthnDetail.FailureReason`. This keeps fallback
// callers (log-only consumers without a `WithErrorHandler` hook) and
// the structured detail consistent.
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

// Named pairs a scheme string with an `Authenticator` for `Multi`. The
// scheme is later written into `AuthnDetail.Method` when this engine is
// the one that succeeded.
//
// The scheme string is opaque to the framework — any value is accepted.
// Engine sub-packages typically expose a `Scheme` constant
// (e.g. `jwt.Scheme = "jwt"`) so business code can write
// `authn.Named(jwt.Scheme, jwt.NewAuthenticator(...))`.
func Named(scheme string, a Authenticator) NamedAuthenticator {
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
// at wiring time). On any success, the winning scheme is written back to
// the package-private holder ctx channel installed by `Server`, which
// then becomes `AuthnDetail.Method`.
//
// If the `allowedSchemes` set installed by `Server` (from
// `Rules.MethodSchemes`) is non-nil, engines whose scheme is absent from
// the set are skipped silently. If nil (unannotated method), every engine
// participates.
//
// Wrapping a single engine via `Multi(Named(...))` is a supported and
// recommended pattern: it gives the dispatcher the scheme name without
// requiring a separate `WithMethod` option, and makes future expansion
// to more engines a one-line change.
func Multi(named ...NamedAuthenticator) Authenticator {
	return &multi{engines: named}
}

// Authenticate iterates engines in injection order; the first to return
// (actor, nil) wins. See `Multi` doc for filter semantics.
func (m *multi) Authenticate(ctx context.Context) (actor.Actor, error) {
	allowed := allowedSchemesFrom(ctx)
	holder := schemeHolderFrom(ctx)

	var attempts []SchemeAttempt
	participated := 0
	for _, named := range m.engines {
		if allowed != nil {
			if _, ok := allowed[named.scheme]; !ok {
				continue
			}
		}
		participated++
		a, err := named.inner.Authenticate(ctx)
		if err == nil {
			// Anonymous-fallthrough guard: a sub-engine that quietly returns
			// (anonymous, nil) — the historical "no credential, treat as
			// anonymous passthrough" pattern from single-engine direct mounts —
			// is treated as a SOFT FAILURE inside Multi. Otherwise the first
			// such engine would short-circuit dispatch and other engines
			// (e.g. apikey) would never run, defeating the multi-scheme intent.
			//
			// Rationale: in a Multi composition the contract is "produce a
			// concrete identity from one of the configured schemes"; anonymous
			// is the absence of identity, not a successful one. Engines that
			// want to allow anonymous fallthrough should do so via a separate
			// non-Multi mount path (or wait for a future explicit option).
			if a == nil || a.Type() == actor.TypeAnonymous {
				attempts = append(attempts, SchemeAttempt{
					Scheme: named.scheme,
					Reason: "no credential (engine returned anonymous)",
				})
				continue
			}
			if holder != nil {
				holder.set(named.scheme)
			}
			return a, nil
		}
		attempts = append(attempts, SchemeAttempt{
			Scheme: named.scheme,
			Reason: err.Error(),
		})
	}

	if participated == 0 {
		return nil, errSchemesEmpty
	}
	return nil, &schemeAttemptsErr{attempts: attempts}
}
