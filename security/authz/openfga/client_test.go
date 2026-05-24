package openfga

import (
	"testing"

	openfgaconfpb "github.com/Servora-Kit/servora/api/gen/go/servora/security/authz/openfga/v1"
)

func TestWithComputedRelations(t *testing.T) {
	m := map[string][]string{"project": {"can_view", "can_edit"}}
	var o clientOptions
	WithComputedRelations(m)(&o)
	if len(o.computedRelations) != 1 {
		t.Fatal("computed relations not set")
	}
	if len(o.computedRelations["project"]) != 2 {
		t.Fatal("computed relations values wrong")
	}
}

func TestWithComputedRelations_Nil(t *testing.T) {
	var o clientOptions
	WithComputedRelations(nil)(&o)
	if o.computedRelations != nil {
		t.Fatal("expected nil computed relations")
	}
}

func TestNewClient_NilConfig(t *testing.T) {
	_, err := NewClient(nil)
	if err == nil {
		t.Fatal("expected error for nil config")
	}
}

func TestNewClient_UsesGeneratedRequiredChecks(t *testing.T) {
	_, err := NewClient(&openfgaconfpb.Config{StoreId: "store"})
	if err == nil {
		t.Fatal("expected missing api_url error")
	}
}
