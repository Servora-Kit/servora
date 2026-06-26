package jwt

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v3/transport"
	gojwt "github.com/golang-jwt/jwt/v5"

	"github.com/Servora-Kit/servora/security/authn"
)

type authenticator struct {
	cfg *authenticatorConfig
}

// NewAuthenticator creates a JWT-based [authn.Authenticator].
//
// If no token is found, Authenticate returns authn.ErrNoCredentials. Missing
// verifier is a wiring error and panics at construction time. Verifier failure
// returns a normal error, so Multi can fail fast.
func NewAuthenticator(opts ...Option) authn.Authenticator {
	cfg := &authenticatorConfig{
		claimsMapper: DefaultClaimsMapper(),
	}
	for _, opt := range opts {
		opt(cfg)
	}
	if cfg.claimsMapper == nil {
		cfg.claimsMapper = DefaultClaimsMapper()
	}
	if cfg.verifier == nil {
		panic("jwt: WithVerifier is required")
	}
	return &authenticator{cfg: cfg}
}

// Authenticate reads the raw bearer token (ctx channel first, transport
// header fallback), verifies it, and returns an enriched context.
func (a *authenticator) Authenticate(ctx context.Context) (context.Context, error) {
	tokenString := tokenForAuth(ctx)
	if tokenString == "" {
		return ctx, authn.ErrNoCredentials
	}
	ctx = WithToken(ctx, tokenString)

	claims := gojwt.MapClaims{}
	if err := a.cfg.verifier.Verify(tokenString, claims); err != nil {
		return ctx, fmt.Errorf("jwt: verify token: %w", err)
	}

	enriched, err := a.cfg.claimsMapper(ctx, claims)
	if err != nil {
		return ctx, err
	}
	return authn.WithAuthType(enriched, Scheme), nil
}

// tokenForAuth resolves the raw bearer token used for verification. The
// jwt-private ctx channel takes precedence over the transport header.
func tokenForAuth(ctx context.Context) string {
	if raw, ok := TokenFrom(ctx); ok && raw != "" {
		return raw
	}
	tr, ok := transport.FromServerContext(ctx)
	if !ok {
		return ""
	}
	return extractBearerToken(tr.RequestHeader().Get("Authorization"))
}
