package apikey

import "context"

// keyMetaKey is the unexported ctx key for [KeyMeta]. This channel is
// package-private — only the apikey engine writes it (on successful
// authentication); downstream code reads via [KeyMetaFrom].
type keyMetaKey struct{}

// WithKeyMeta attaches a [KeyMeta] to ctx. Called by the apikey
// authenticator on success; business code should not need to call this
// directly.
func WithKeyMeta(ctx context.Context, meta KeyMeta) context.Context {
	return context.WithValue(ctx, keyMetaKey{}, meta)
}

// KeyMetaFrom retrieves the [KeyMeta] previously set via [WithKeyMeta].
// Returns the zero value and false if not present.
func KeyMetaFrom(ctx context.Context) (KeyMeta, bool) {
	m, ok := ctx.Value(keyMetaKey{}).(KeyMeta)
	return m, ok
}
