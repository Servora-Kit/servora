package middleware

import (
	"context"
	"strings"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"

	"github.com/Servora-Kit/servora/pkg/actor"
)

// Default header names injected by the gateway after identity verification.
const (
	DefaultUserIDHeader        = "X-User-ID"
	DefaultSubjectHeader       = "X-Subject"
	DefaultClientIDHeader      = "X-Client-ID"
	DefaultRealmHeader         = "X-Realm"
	DefaultEmailHeader         = "X-Email"
	DefaultRolesHeader         = "X-Roles"   // comma-separated
	DefaultScopesHeader        = "X-Scopes"  // space-separated
	DefaultPrincipalTypeHeader = "X-Principal-Type"
)

// HeaderMapping maps semantic actor fields to actual HTTP header names.
// Override individual keys with WithHeaderMapping to adapt to different gateway conventions.
type HeaderMapping struct {
	UserID        string
	Subject       string
	ClientID      string
	Realm         string
	Email         string
	Roles         string
	Scopes        string
	PrincipalType string
}

func defaultHeaderMapping() HeaderMapping {
	return HeaderMapping{
		UserID:        DefaultUserIDHeader,
		Subject:       DefaultSubjectHeader,
		ClientID:      DefaultClientIDHeader,
		Realm:         DefaultRealmHeader,
		Email:         DefaultEmailHeader,
		Roles:         DefaultRolesHeader,
		Scopes:        DefaultScopesHeader,
		PrincipalType: DefaultPrincipalTypeHeader,
	}
}

// IdentityOption configures the IdentityFromHeader middleware.
type IdentityOption func(*identityConfig)

type identityConfig struct {
	mapping HeaderMapping
}

// WithHeaderKey overrides the X-User-ID header name (legacy single-header shorthand).
func WithHeaderKey(key string) IdentityOption {
	return func(c *identityConfig) { c.mapping.UserID = key }
}

// WithHeaderMapping overrides specific header keys. Zero-value fields in m are ignored
// (keep defaults). Example:
//
//	WithHeaderMapping(HeaderMapping{Roles: "X-Custom-Roles"})
func WithHeaderMapping(m HeaderMapping) IdentityOption {
	return func(c *identityConfig) {
		if m.UserID != "" {
			c.mapping.UserID = m.UserID
		}
		if m.Subject != "" {
			c.mapping.Subject = m.Subject
		}
		if m.ClientID != "" {
			c.mapping.ClientID = m.ClientID
		}
		if m.Realm != "" {
			c.mapping.Realm = m.Realm
		}
		if m.Email != "" {
			c.mapping.Email = m.Email
		}
		if m.Roles != "" {
			c.mapping.Roles = m.Roles
		}
		if m.Scopes != "" {
			c.mapping.Scopes = m.Scopes
		}
		if m.PrincipalType != "" {
			c.mapping.PrincipalType = m.PrincipalType
		}
	}
}

// IdentityFromHeader creates a Kratos middleware that reads the user identity
// from gateway-injected HTTP headers and injects an actor.Actor into the request context.
//
// This is the lightweight counterpart of a full JWT Authn middleware: it trusts
// that the gateway has already performed token verification and simply propagates
// the resulting identity via headers (gateway-agnostic, works with any proxy).
//
// Supported headers (all configurable via WithHeaderMapping):
//   - X-User-ID           → actor ID
//   - X-Subject           → IdP subject (Keycloak sub)
//   - X-Client-ID         → OAuth2 client_id
//   - X-Realm             → IdP realm
//   - X-Email             → user email
//   - X-Roles             → comma-separated role list
//   - X-Scopes            → space-separated OAuth2 scopes
//   - X-Principal-Type    → "user" (default) or "service"
//
// If X-Principal-Type is "service", a ServiceActor is injected instead of UserActor.
// If X-User-ID is absent, AnonymousActor is injected.
func IdentityFromHeader(opts ...IdentityOption) middleware.Middleware {
	cfg := &identityConfig{mapping: defaultHeaderMapping()}
	for _, o := range opts {
		o(cfg)
	}

	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			tr, ok := transport.FromServerContext(ctx)
			if !ok {
				ctx = actor.NewContext(ctx, actor.NewAnonymousActor())
				return handler(ctx, req)
			}

			h := tr.RequestHeader()
			m := cfg.mapping

			userID := h.Get(m.UserID)
			if userID == "" {
				ctx = actor.NewContext(ctx, actor.NewAnonymousActor())
				return handler(ctx, req)
			}

			principalType := strings.ToLower(h.Get(m.PrincipalType))
			if principalType == "service" {
				svc := actor.NewServiceActor(userID, h.Get(m.ClientID), h.Get(m.Subject))
				svc.SetRealm(h.Get(m.Realm))
				svc.SetScopes(splitScopes(h.Get(m.Scopes)))
				ctx = actor.NewContext(ctx, svc)
				return handler(ctx, req)
			}

			ua := actor.NewUserActor(actor.UserActorParams{
				ID:          userID,
				Subject:     h.Get(m.Subject),
				ClientID:    h.Get(m.ClientID),
				Realm:       h.Get(m.Realm),
				Email:       h.Get(m.Email),
				Roles:       splitRoles(h.Get(m.Roles)),
				Scopes:      splitScopes(h.Get(m.Scopes)),
			})
			ctx = actor.NewContext(ctx, ua)
			return handler(ctx, req)
		}
	}
}

// splitRoles splits a comma-separated roles header value.
func splitRoles(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// splitScopes splits a space-separated scopes header value.
func splitScopes(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Fields(s)
}
