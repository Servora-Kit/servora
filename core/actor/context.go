package actor

import "context"

type contextKey struct{}

// NewContext returns a new context carrying the given Actor.
func NewContext(ctx context.Context, a Actor) context.Context {
	return context.WithValue(ctx, contextKey{}, a)
}

// From extracts the Actor previously stored via NewContext.
func From(ctx context.Context) (Actor, bool) {
	a, ok := ctx.Value(contextKey{}).(Actor)
	return a, ok
}

// MustFrom panics if no actor is in context — use only in trusted code paths.
func MustFrom(ctx context.Context) Actor {
	a, ok := From(ctx)
	if !ok {
		panic("actor: no actor in context")
	}
	return a
}
