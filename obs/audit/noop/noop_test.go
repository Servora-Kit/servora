package noop

import (
	"context"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/Servora-Kit/servora/obs/audit"
)

func TestNewAuditor_ImplementsInterface(t *testing.T) {
	var _ audit.Auditor = NewAuditor() //nolint:staticcheck // compile-time interface assertion
}

func TestAuditor_EmitReturnsNil(t *testing.T) {
	a := NewAuditor()
	e := cloudevents.NewEvent()
	e.SetType("test")
	e.SetSource("test")

	err := a.Emit(context.Background(), e)
	if err != nil {
		t.Errorf("Emit() returned error: %v", err)
	}
}
