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
package authn

import (
	"context"
	"strings"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"

	"github.com/Servora-Kit/servora/core/actor"
	svrmw "github.com/Servora-Kit/servora/transport/server/middleware"
)

// Authenticator is the interface for authenticating incoming requests.
// Implementations receive the full request context (which may include
// the raw token stored by Server) and return an actor.Actor.
type Authenticator interface {
	Authenticate(ctx context.Context) (actor.Actor, error)
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
//   - No transport in context → anonymous actor injected, handler called
//   - No Authorization header → anonymous actor injected (authenticator may override)
//   - Authenticator error + no error handler → error returned
//   - Authenticator error + error handler → handler's return value used
func Server(authenticator Authenticator, opts ...Option) middleware.Middleware {
	cfg := &serverConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			tr, ok := transport.FromServerContext(ctx)
			if !ok {
				ctx = actor.NewContext(ctx, actor.NewAnonymousActor())
				return handler(ctx, req)
			}

			// Extract raw token and store for downstream consumers (e.g. opaque pass-through).
			if tokenString := ExtractBearerToken(tr.RequestHeader().Get("Authorization")); tokenString != "" {
				ctx = svrmw.NewTokenContext(ctx, tokenString)
			}

			a, err := authenticator.Authenticate(ctx)
			if err != nil {
				if cfg.errorHandler != nil {
					return nil, cfg.errorHandler(ctx, err)
				}
				return nil, err
			}

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
