package apikey

import (
	"context"
	"errors"

	"github.com/Servora-Kit/servora/core/actor"
	"github.com/Servora-Kit/servora/security/authn"
)

// Ensure *authenticator implements authn.Authenticator at compile time.
var _ authn.Authenticator = (*authenticator)(nil)

// errMissingHeader is returned when the inbound request carries no
// `X-API-Key` header (or no server transport is attached to ctx). The
// string is matched by tests; downstream callers SHOULD NOT rely on its
// exact wording but the substring "missing X-API-Key" is stable.
var errMissingHeader = errors.New("apikey: missing X-API-Key header")

// NewAuthenticator creates an API-key based [authn.Authenticator].
//
// The returned authenticator's `Authenticate(ctx)`:
//
//  1. Reads the `X-API-Key` header from the Kratos server transport.
//  2. If absent / empty → returns `(nil, errMissingHeader)`.
//  3. Otherwise calls `Store.Lookup(ctx, key)` and propagates its
//     `(actor.Actor, error)` verbatim.
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
// dispatches to the configured [Store]. Returns `(nil, errMissingHeader)`
// when the header is absent; otherwise propagates `Store.Lookup` verbatim.
func (a *authenticator) Authenticate(ctx context.Context) (actor.Actor, error) {
	key := extractAPIKey(ctx)
	if key == "" {
		return nil, errMissingHeader
	}
	return a.store.Lookup(ctx, key)
}
