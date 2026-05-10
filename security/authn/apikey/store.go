package apikey

import (
	"context"

	"github.com/Servora-Kit/servora/core/actor"
)

// Store resolves an API key string into an [actor.Actor].
//
// Implementations may be backed by an in-memory map (test stubs), a
// database table, a cache, or a cross-service RPC. The framework imposes
// no constraint on the actor type — implementations should construct an
// appropriate concrete (e.g. `*actor.UserActor` for human-issued keys,
// `*actor.ServiceActor` for service-account keys, or any custom actor
// implementation).
//
// Error semantics:
//
//   - Unknown / revoked / disabled keys SHOULD return a non-nil error;
//     the apikey engine propagates that error verbatim to the dispatcher.
//   - The error string is surfaced via `*auditpb.AuthnDetail.FailureReason`
//     when the request ultimately fails, so implementations should keep
//     reasons short and PII-free.
type Store interface {
	Lookup(ctx context.Context, key string) (actor.Actor, error)
}
