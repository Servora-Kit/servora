package authn

import (
	"context"
	"fmt"
	"strings"

	gojwt "github.com/golang-jwt/jwt/v5"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"

	"github.com/Servora-Kit/servora/pkg/actor"
	jwtpkg "github.com/Servora-Kit/servora/pkg/jwt"
	svrmw "github.com/Servora-Kit/servora/pkg/transport/server/middleware"
)

// UserClaimsMapper converts parsed JWT MapClaims into an actor.Actor.
type UserClaimsMapper func(claims gojwt.MapClaims) (actor.Actor, error)

// Option configures the Authn middleware.
type Option func(*options)

type options struct {
	verifier     *jwtpkg.Verifier
	claimsMapper UserClaimsMapper
	errorHandler func(ctx context.Context, err error) error
}

// WithVerifier sets the JWT verifier. If nil, the middleware operates in pass-through mode.
func WithVerifier(v *jwtpkg.Verifier) Option {
	return func(o *options) { o.verifier = v }
}

// WithClaimsMapper sets a custom function to map JWT claims to an actor.
func WithClaimsMapper(m UserClaimsMapper) Option {
	return func(o *options) { o.claimsMapper = m }
}

// WithErrorHandler sets a custom error handler invoked when token verification or claims mapping fails.
func WithErrorHandler(h func(ctx context.Context, err error) error) Option {
	return func(o *options) { o.errorHandler = h }
}

func defaultClaimsMapper(claims gojwt.MapClaims) (actor.Actor, error) {
	sub := claimString(claims, "sub")
	id := sub
	if id == "" {
		id = claimString(claims, "id")
	}

	// Merge both "roles" (array) and "role" (string, legacy) claims.
	roles := claimStringSlice(claims, "roles")
	if singleRole := claimString(claims, "role"); singleRole != "" {
		roles = append(roles, singleRole)
	}

	// Build open attrs for any extra claims not captured by named fields.
	attrs := make(map[string]string)
	if role := claimString(claims, "role"); role != "" {
		attrs["role"] = role
	}

	return actor.NewUserActor(actor.UserActorParams{
		ID:          id,
		DisplayName: claimString(claims, "name"),
		Email:       claimString(claims, "email"),
		Subject:     sub,
		ClientID:    claimString(claims, "azp"),  // Keycloak authorized party
		Realm:       claimString(claims, "iss"),  // issuer as realm hint
		Roles:       roles,
		Scopes:      claimStringSlice(claims, "scope"),
		Attrs:       attrs,
	}), nil
}

// claimStringSlice extracts a string slice from a claim (handles both []interface{} and
// space-separated string formats).
func claimStringSlice(claims gojwt.MapClaims, key string) []string {
	v, ok := claims[key]
	if !ok {
		return nil
	}
	switch val := v.(type) {
	case []any:
		out := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case string:
		if val == "" {
			return nil
		}
		parts := strings.Fields(val)
		return parts
	default:
		return nil
	}
}

func claimString(claims gojwt.MapClaims, key string) string {
	v, ok := claims[key]
	if !ok {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return fmt.Sprintf("%.0f", val)
	default:
		return fmt.Sprintf("%v", val)
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

// Authn creates a JWT token verification middleware that injects an actor.Actor into the
// request context. Anonymous actor is injected when no token is present.
// Use with selector.Server + WhiteList to expose unauthenticated routes.
func Authn(opts ...Option) middleware.Middleware {
	cfg := &options{
		claimsMapper: defaultClaimsMapper,
	}
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

			tokenString := ExtractBearerToken(tr.RequestHeader().Get("Authorization"))
			if tokenString == "" {
				ctx = actor.NewContext(ctx, actor.NewAnonymousActor())
				return handler(ctx, req)
			}

			if cfg.verifier == nil {
				ctx = actor.NewContext(ctx, actor.NewAnonymousActor())
				ctx = svrmw.NewTokenContext(ctx, tokenString)
				return handler(ctx, req)
			}

			claims := gojwt.MapClaims{}
			if err := cfg.verifier.Verify(tokenString, claims); err != nil {
				if cfg.errorHandler != nil {
					return nil, cfg.errorHandler(ctx, err)
				}
				return nil, err
			}

			a, err := cfg.claimsMapper(claims)
			if err != nil {
				if cfg.errorHandler != nil {
					return nil, cfg.errorHandler(ctx, err)
				}
				return nil, err
			}

			ctx = actor.NewContext(ctx, a)
			ctx = svrmw.NewTokenContext(ctx, tokenString)
			return handler(ctx, req)
		}
	}
}
