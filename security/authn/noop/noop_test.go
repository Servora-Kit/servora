package noop

import (
	"context"
	"testing"

	"github.com/Servora-Kit/servora/security/authn"
)

var _ authn.Authenticator = (*Authenticator)(nil)

func TestNewAuthenticator_ReturnsImplementation(t *testing.T) {
	t.Parallel()
	a := NewAuthenticator()
	if a == nil {
		t.Fatal("expected non-nil Authenticator, got nil")
	}
}

func TestAuthenticate_ReturnsSameContext(t *testing.T) {
	t.Parallel()
	a := NewAuthenticator()
	ctx := context.Background()
	got, err := a.Authenticate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != ctx {
		t.Error("expected same context back (noop passthrough)")
	}
}
