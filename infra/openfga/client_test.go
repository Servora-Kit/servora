package openfga

import (
	"testing"

	"github.com/Servora-Kit/servora/obs/audit"
)

func TestWithAuditRecorder(t *testing.T) {
	var o clientOptions
	r := audit.NewRecorder(nil, "test")
	WithAuditRecorder(r)(&o)
	if o.recorder != r {
		t.Fatal("recorder not set")
	}
}

func TestWithAuditRecorder_Nil(t *testing.T) {
	var o clientOptions
	WithAuditRecorder(nil)(&o)
	if o.recorder != nil {
		t.Fatal("expected nil recorder")
	}
}

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
