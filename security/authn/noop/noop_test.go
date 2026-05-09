package noop

import (
	"context"
	"testing"

	"github.com/Servora-Kit/servora/core/actor"
	"github.com/Servora-Kit/servora/security/authn"
)

// Compile-time: *Authenticator implements the single-method authn.Authenticator
// interface. Mirrors the assertion in noop.go so a regression here surfaces in
// the test binary as well.
var _ authn.Authenticator = (*Authenticator)(nil)

func TestNewAuthenticator_ReturnsImplementation(t *testing.T) {
	t.Parallel()

	a := NewAuthenticator()
	if a == nil {
		t.Fatal("expected non-nil Authenticator, got nil")
	}
	// The compile-time assertion above already proves interface satisfaction;
	// this subtest documents the runtime contract for human readers.
}

func TestAuthenticate_ReturnsAnonymousActor(t *testing.T) {
	t.Parallel()

	a := NewAuthenticator()
	got, err := a.Authenticate(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil actor")
	}
	if got.Type() != actor.TypeAnonymous {
		t.Errorf("expected anonymous actor type %q, got %q", actor.TypeAnonymous, got.Type())
	}
}

// Negative interface check: ensure Authenticator does NOT satisfy a hypothetical
// interface that includes Method(). This acts as a regression guard for future
// drift toward re-introducing engine metadata onto the interface.
func TestAuthenticator_DoesNotExposeMethod(t *testing.T) {
	t.Parallel()

	type withMethod interface {
		Method() string
	}
	a := NewAuthenticator()
	if _, ok := a.(withMethod); ok {
		t.Error("noop.Authenticator unexpectedly satisfies an interface with Method() — Task 2/3 should have removed it")
	}
}
