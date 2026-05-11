package apikey

import "context"

// KeyMeta carries the minimal metadata that the framework needs from a
// resolved API key. Business code can define its own Store returning
// richer structures; the framework only inspects these two fields.
//
//   - KeyID:   the key identifier (NOT the secret); used in audit trails.
//   - OwnerID: the subject identifier of the key owner; surfaced via
//     [SubjectFrom].
type KeyMeta struct {
	KeyID   string // The key identifier (not the secret)
	OwnerID string // Owner subject identifier
}

// Store resolves an API key string into a [KeyMeta].
//
// Implementations may be backed by an in-memory map (test stubs), a
// database table, a cache, or a cross-service RPC. The framework imposes
// no constraint on how keys are stored — implementations construct a
// [KeyMeta] with the identifier and owner of the resolved key.
//
// Error semantics:
//
//   - Unknown / revoked / disabled keys SHOULD return a non-nil error;
//     the apikey engine propagates that error verbatim to the dispatcher.
//   - The error string is surfaced via audit failure reason when the
//     request ultimately fails, so implementations should keep reasons
//     short and PII-free.
type Store interface {
	Lookup(ctx context.Context, key string) (KeyMeta, error)
}
