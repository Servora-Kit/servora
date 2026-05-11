package multi

import (
	"context"
	"errors"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/Servora-Kit/servora/obs/audit"
)

type countAuditor struct {
	count int
	err   error
}

func (c *countAuditor) Emit(_ context.Context, _ cloudevents.Event) error {
	c.count++
	return c.err
}

func TestMulti_FanOut(t *testing.T) {
	a1 := &countAuditor{}
	a2 := &countAuditor{}
	a3 := &countAuditor{}

	m := New(a1, a2, a3)

	e := cloudevents.NewEvent()
	e.SetType("test")
	e.SetSource("test")

	err := m.Emit(context.Background(), e)
	if err != nil {
		t.Errorf("Emit() returned error: %v", err)
	}

	for i, a := range []*countAuditor{a1, a2, a3} {
		if a.count != 1 {
			t.Errorf("auditor %d: count = %d, want 1", i, a.count)
		}
	}
}

func TestMulti_PartialFailureDoesNotBlock(t *testing.T) {
	failErr := errors.New("fail")
	a1 := &countAuditor{}
	a2 := &countAuditor{err: failErr}
	a3 := &countAuditor{}

	m := New(a1, a2, a3)

	e := cloudevents.NewEvent()
	e.SetType("test")
	e.SetSource("test")

	err := m.Emit(context.Background(), e)

	// All auditors should have been called despite a2 failing.
	if a1.count != 1 {
		t.Errorf("a1 count = %d, want 1", a1.count)
	}
	if a2.count != 1 {
		t.Errorf("a2 count = %d, want 1", a2.count)
	}
	if a3.count != 1 {
		t.Errorf("a3 count = %d, want 1", a3.count)
	}

	// Error should be reported.
	if err == nil {
		t.Error("expected error from partial failure")
	}
	if !errors.Is(err, failErr) {
		t.Errorf("expected error to contain failErr, got: %v", err)
	}
}

func TestMulti_Empty(t *testing.T) {
	m := New()
	e := cloudevents.NewEvent()
	e.SetType("test")
	e.SetSource("test")

	err := m.Emit(context.Background(), e)
	if err != nil {
		t.Errorf("empty multi Emit() returned error: %v", err)
	}
}

func TestMulti_ImplementsInterface(t *testing.T) {
	var _ audit.Auditor = New()
}
