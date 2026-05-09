// Package authn provides an engine-agnostic Kratos middleware dispatcher for
// authentication. The main package is a pure dispatcher: it knows nothing
// about credential carriers (Bearer token, mTLS PeerCertificate, API-Key
// header, signature, etc.). Engine sub-packages (e.g. `security/authn/jwt`)
// are responsible for credential I/O and delegate to `authn.Server` with the
// engine method string supplied via `authn.WithMethod(...)`.
//
// Example: business code uses the engine wrapper, not the main package
// directly.
//
//	import (
//	    "github.com/Servora-Kit/servora/security/authn/jwt"
//	)
//
//	mw = append(mw, jwt.Server(jwt.WithVerifier(km.Verifier())))
//
// Power-user / custom-engine direct call:
//
//	mw = append(mw, authn.Server(myAuth, authn.WithMethod("passkey")))
//
// The middleware writes a *auditpb.AuthnDetail to ctx via
// audit.WithAuthnResult; emission is the responsibility of the transport-tail
// audit.Collector middleware. The authn package therefore has zero coupling
// to the audit emission pipeline (only to the neutral auditpb schema package).
package authn

import (
	"context"

	"github.com/go-kratos/kratos/v2/middleware"

	auditpb "github.com/Servora-Kit/servora/api/gen/go/servora/audit/v1"
	"github.com/Servora-Kit/servora/core/actor"
	"github.com/Servora-Kit/servora/obs/audit"
)

// Authenticator is the interface for authenticating incoming requests.
//
// CONTRACT: this interface intentionally contains ONLY behavior body
// (`Authenticate`). Engine metadata (the method string used in audit
// detail) belongs to the wrapper layer via the `authn.WithMethod(...)`
// option — NOT on the interface itself.
//
// What MUST NOT live on this interface:
//   - Engine metadata (e.g. `Method() string`) — supplied by wrapper through
//     `WithMethod` option; framework main package is agnostic to the string.
//   - Hooks / callbacks (e.g. `OnSuccess`) — caller responsibility.
//   - Injection (logger / tracer) — container responsibility.
//   - Infra probes (e.g. `Health`) — separate sibling interface.
//
// This single-method shape prevents interface bloat as new engines (mTLS,
// API-Key, AK+SK, Passkey, etc.) are added: each engine is described by
// the wrapper's package-private `methodName` constant, and orchestration
// is the middleware's responsibility.
type Authenticator interface {
	Authenticate(ctx context.Context) (actor.Actor, error)
}

// Option configures the Server middleware.
type Option func(*serverConfig)

type serverConfig struct {
	method       string
	errorHandler func(ctx context.Context, err error) error
}

// WithMethod sets the engine method string written into
// `*auditpb.AuthnDetail.Method` on every dispatch. Wrapper sub-packages
// SHALL always pass this option (using a package-private constant such as
// `const methodName = "jwt"`); business code calling `authn.Server(...)`
// directly MUST also pass it.
//
// The framework main package is agnostic to the string contents — any value
// is accepted; missing/empty is allowed but discouraged.
func WithMethod(m string) Option {
	return func(c *serverConfig) { c.method = m }
}

// WithErrorHandler sets a custom error handler invoked when authentication fails.
func WithErrorHandler(h func(ctx context.Context, err error) error) Option {
	return func(c *serverConfig) { c.errorHandler = h }
}

// Server returns a Kratos middleware that dispatches authentication to the
// supplied Authenticator. It is engine-agnostic: it does not read transport
// headers, parse credentials, or write any engine-specific ctx channel —
// those concerns belong to engine wrapper sub-packages (e.g. `jwt.Server`).
//
// Behavior:
//   - Calls `authenticator.Authenticate(ctx)` directly; the engine is
//     expected to read whatever credential channel its wrapper installed.
//   - On success: writes a Success=true `*auditpb.AuthnDetail` to ctx via
//     `audit.WithAuthnResult`, injects the returned actor via
//     `actor.NewContext`, and calls the handler.
//   - On error: writes a Success=false `*auditpb.AuthnDetail` (with
//     `FailureReason = err.Error()`) to ctx BEFORE returning. An outer
//     `audit.Collector` middleware (mounted in front of `Server`) sees the
//     ctx-bound detail in its post-phase and emits an AUTHN_RESULT event
//     even when authn short-circuits. If `WithErrorHandler` was supplied,
//     its return value replaces the raw error.
//
// `AuthnDetail.Method` is filled from the string supplied via `WithMethod`.
// The middleware does NOT introspect the authenticator for self-description
// (the historical `Method` accessor has been removed from the interface).
func Server(authenticator Authenticator, opts ...Option) middleware.Middleware {
	cfg := &serverConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	// Cache method outside the closure: it is supplied at construction time
	// and immutable per middleware instance.
	method := cfg.method

	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
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
