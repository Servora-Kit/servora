package openfga

import (
	"context"
	"testing"

	"github.com/Servora-Kit/servora/security/authz"
	"github.com/Servora-Kit/servora/security/authz/batch"
	"github.com/Servora-Kit/servora/security/authz/lister"
)

// Compile-time interface assertions — if these fail, the package won't compile.
var (
	_ authz.Authorizer      = (*Authorizer)(nil)
	_ batch.BatchAuthorizer  = (*Authorizer)(nil)
	_ lister.Lister          = (*Authorizer)(nil)
)

func TestAuthorizer_ImplementsAuthzAuthorizer(t *testing.T) {
	// Type assertion at runtime to verify interface satisfaction.
	var a any = &Authorizer{}
	if _, ok := a.(authz.Authorizer); !ok {
		t.Fatal("*Authorizer does not implement authz.Authorizer")
	}
}

func TestAuthorizer_ImplementsBatchAuthorizer(t *testing.T) {
	var a any = &Authorizer{}
	if _, ok := a.(batch.BatchAuthorizer); !ok {
		t.Fatal("*Authorizer does not implement batch.BatchAuthorizer")
	}
}

func TestAuthorizer_ImplementsLister(t *testing.T) {
	var a any = &Authorizer{}
	if _, ok := a.(lister.Lister); !ok {
		t.Fatal("*Authorizer does not implement lister.Lister")
	}
}

func TestNewAuthorizer_DefaultTTL(t *testing.T) {
	a := NewAuthorizer(&Client{})
	if a.ttl != DefaultCheckCacheTTL {
		t.Errorf("default TTL = %v, want %v", a.ttl, DefaultCheckCacheTTL)
	}
	if a.redis != nil {
		t.Error("redis should be nil by default")
	}
}

func TestNewAuthorizer_WithRedisCache_NilClient(t *testing.T) {
	a := NewAuthorizer(&Client{}, WithRedisCache(nil, 0))
	if a.redis != nil {
		t.Error("redis should remain nil when passed nil")
	}
}

func TestBatchCheck_EmptyInput(t *testing.T) {
	a := NewAuthorizer(&Client{})
	got, err := a.BatchCheck(context.TODO(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
}
