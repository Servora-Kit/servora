package audit

import (
	"context"
	"errors"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

func TestMulti_FanOut(t *testing.T) {
	a1 := &mockAuditor{}
	a2 := &mockAuditor{}

	m := Multi(a1, a2)

	e := cloudevents.NewEvent()
	e.SetType("test")
	e.SetSource("test")

	err := m.Emit(context.Background(), e)
	if err != nil {
		t.Fatalf("Multi.Emit() returned error: %v", err)
	}

	if len(a1.Events()) != 1 {
		t.Errorf("a1 got %d events, want 1", len(a1.Events()))
	}
	if len(a2.Events()) != 1 {
		t.Errorf("a2 got %d events, want 1", len(a2.Events()))
	}
}

func TestMulti_PartialFailure(t *testing.T) {
	failErr := errors.New("emit fail")
	a1 := &mockAuditor{}
	a2 := &mockAuditor{err: failErr}
	a3 := &mockAuditor{}

	m := Multi(a1, a2, a3)

	e := cloudevents.NewEvent()
	e.SetType("test")
	e.SetSource("test")

	err := m.Emit(context.Background(), e)
	if !errors.Is(err, failErr) {
		t.Errorf("expected failErr, got: %v", err)
	}

	// All auditors should have been called.
	if len(a1.Events()) != 1 {
		t.Errorf("a1 got %d events, want 1", len(a1.Events()))
	}
	if len(a3.Events()) != 1 {
		t.Errorf("a3 got %d events, want 1", len(a3.Events()))
	}
}
