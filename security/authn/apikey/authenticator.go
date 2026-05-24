package apikey

import (
	"context"
	"fmt"

	"github.com/Servora-Kit/servora/security/authn"
)

// Ensure *authenticator implements authn.Authenticator at compile time.
var _ authn.Authenticator = (*authenticator)(nil)

// errMissingHeader is returned when the inbound request carries no
// `X-API-Key` header or no server transport is attached to ctx.
var errMissingHeader = fmt.Errorf("apikey: missing X-API-Key header: %w", authn.ErrNoCredentials)

// NewAuthenticator creates an API-key based [authn.Authenticator].
//
// The returned authenticator's `Authenticate(ctx)`:
//
//  1. Reads the `X-API-Key` header from the Kratos server transport.
//  2. If absent / empty → returns `(ctx, errMissingHeader)`, which matches
//     [authn.ErrNoCredentials].
//  3. Otherwise calls `Store.Lookup(ctx, key)`.
//  4. On success: attaches [KeyMeta] via [WithKeyMeta] and sets
//     auth type to "api_key" via [authn.WithAuthType]; returns the
//     enriched ctx.
//  5. On failure: propagates Store.Lookup error verbatim.
//
// REQUIRED: at least one [WithStore] Option MUST be supplied. Calling
// `NewAuthenticator()` (no opts) panics with `apikey: WithStore is
// required`. The fail-fast panic surfaces wiring bugs at boot time
// rather than per-request 401s.
func NewAuthenticator(opts ...Option) authn.Authenticator {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}
	if cfg.store == nil {
		panic("apikey: WithStore is required")
	}
	return &authenticator{store: cfg.store}
}

type authenticator struct {
	store Store
}

// Authenticate reads the X-API-Key header off the inbound transport and
// dispatches to the configured [Store]. Returns `(ctx, errMissingHeader)`
// when the header is absent; on success attaches [KeyMeta] and auth type
// to ctx; on Store error propagates verbatim.
func (a *authenticator) Authenticate(ctx context.Context) (context.Context, error) {
	key := extractAPIKey(ctx)
	if key == "" {
		return ctx, errMissingHeader
	}
	meta, err := a.store.Lookup(ctx, key)
	if err != nil {
		return ctx, err
	}
	ctx = WithKeyMeta(ctx, meta)
	ctx = authn.WithAuthType(ctx, "api_key")
	return ctx, nil
}
