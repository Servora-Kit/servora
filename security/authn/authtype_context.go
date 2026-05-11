package authn

import "context"

// authTypeKey is the unexported ctx key for the authentication type string.
// This channel allows engine implementations to annotate the context with
// the auth mechanism used (e.g. "jwt", "apikey", "mtls") without coupling
// to the Authenticator interface signature.
type authTypeKey struct{}

// WithAuthType attaches an authentication type string to ctx. Engines call
// this on success to communicate the mechanism used upstream.
func WithAuthType(ctx context.Context, authType string) context.Context {
	return context.WithValue(ctx, authTypeKey{}, authType)
}

// AuthTypeFrom retrieves the authentication type string previously set via
// WithAuthType. Returns ("", false) if not present.
func AuthTypeFrom(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(authTypeKey{}).(string)
	return v, ok
}
