package multi

import (
	"context"
	"errors"
	"testing"

	"github.com/Servora-Kit/servora/obs/audit"
	cloudevents "github.com/cloudevents/sdk-go/v2"
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
	var _ audit.Auditor = New() //nolint:staticcheck // compile-time interface assertion
}

// closeAuditor is an Auditor that also implements audit.Closer.
type closeAuditor struct {
	countAuditor
	closed   bool
	closeErr error
}

func (c *closeAuditor) Close() error {
	c.closed = true
	return c.closeErr
}

// flushAuditor is an Auditor that also implements audit.Flusher.
type flushAuditor struct {
	countAuditor
	flushed  bool
	flushErr error
}

func (f *flushAuditor) Flush(_ context.Context) error {
	f.flushed = true
	return f.flushErr
}

func TestMulti_ClosePropagated(t *testing.T) {
	c1 := &closeAuditor{}
	c2 := &closeAuditor{}
	plain := &countAuditor{} // does not implement Closer

	m := New(c1, plain, c2)
	closer, ok := m.(audit.Closer)
	if !ok {
		t.Fatal("multiAuditor does not implement audit.Closer")
	}

	if err := closer.Close(); err != nil {
		t.Errorf("Close() returned unexpected error: %v", err)
	}
	if !c1.closed {
		t.Error("c1.Close() was not called")
	}
	if !c2.closed {
		t.Error("c2.Close() was not called")
	}
}

func TestMulti_CloseErrorPropagated(t *testing.T) {
	closeErr := errors.New("close fail")
	c1 := &closeAuditor{closeErr: closeErr}
	c2 := &closeAuditor{}

	m := New(c1, c2)
	closer := m.(audit.Closer)

	err := closer.Close()
	if !errors.Is(err, closeErr) {
		t.Errorf("expected closeErr, got: %v", err)
	}
	// c2 must still be closed even though c1 failed.
	if !c2.closed {
		t.Error("c2.Close() was not called despite c1 error")
	}
}

func TestMulti_FlushPropagated(t *testing.T) {
	f1 := &flushAuditor{}
	f2 := &flushAuditor{}
	plain := &countAuditor{} // does not implement Flusher

	m := New(f1, plain, f2)
	flusher, ok := m.(audit.Flusher)
	if !ok {
		t.Fatal("multiAuditor does not implement audit.Flusher")
	}

	if err := flusher.Flush(context.Background()); err != nil {
		t.Errorf("Flush() returned unexpected error: %v", err)
	}
	if !f1.flushed {
		t.Error("f1.Flush() was not called")
	}
	if !f2.flushed {
		t.Error("f2.Flush() was not called")
	}
}

func TestMulti_FlushErrorPropagated(t *testing.T) {
	flushErr := errors.New("flush fail")
	f1 := &flushAuditor{flushErr: flushErr}
	f2 := &flushAuditor{}

	m := New(f1, f2)
	flusher := m.(audit.Flusher)

	err := flusher.Flush(context.Background())
	if !errors.Is(err, flushErr) {
		t.Errorf("expected flushErr, got: %v", err)
	}
	// f2 must still be flushed even though f1 failed.
	if !f2.flushed {
		t.Error("f2.Flush() was not called despite f1 error")
	}
}

func TestMulti_NonCloserNonFlusherSkipped(t *testing.T) {
	// Plain auditors that do not implement Closer or Flusher must not cause
	// a panic or error when Close/Flush is called on the multi auditor.
	plain := &countAuditor{}
	m := New(plain)

	closer := m.(audit.Closer)
	if err := closer.Close(); err != nil {
		t.Errorf("Close() on non-Closer backend returned error: %v", err)
	}

	flusher := m.(audit.Flusher)
	if err := flusher.Flush(context.Background()); err != nil {
		t.Errorf("Flush() on non-Flusher backend returned error: %v", err)
	}
}
