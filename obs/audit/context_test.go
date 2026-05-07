package audit

import (
	"context"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	auditpb "github.com/Servora-Kit/servora/api/gen/go/servora/audit/v1"
)

// TestWithAuthnResult_RoundTrip checks that detail written into a holder-bearing
// ctx can be retrieved via AuthnResultFrom.
func TestWithAuthnResult_RoundTrip(t *testing.T) {
	ctx := InstallHolder(context.Background())
	want := &auditpb.AuthnDetail{Method: "jwt", Success: true}
	ctx = WithAuthnResult(ctx, want)

	got, ok := AuthnResultFrom(ctx)
	if !ok {
		t.Fatal("AuthnResultFrom returned ok=false after WithAuthnResult")
	}
	if got != want {
		t.Errorf("got %p, want same pointer %p", got, want)
	}
	if got.Method != "jwt" || !got.Success {
		t.Errorf("retrieved detail mismatch: %+v", got)
	}
}

// TestAuthnResultFrom_Empty returns (nil, false) when no detail was written
// (holder installed but never written to).
func TestAuthnResultFrom_Empty(t *testing.T) {
	ctx := InstallHolder(context.Background())
	got, ok := AuthnResultFrom(ctx)
	if ok {
		t.Errorf("ok=true on empty holder, got %+v", got)
	}
	if got != nil {
		t.Errorf("got non-nil detail on empty holder: %+v", got)
	}
}

// TestAuthnResultFrom_NoHolder returns (nil, false) when no holder is installed.
func TestAuthnResultFrom_NoHolder(t *testing.T) {
	got, ok := AuthnResultFrom(context.Background())
	if ok {
		t.Errorf("ok=true on bare ctx, got %+v", got)
	}
	if got != nil {
		t.Errorf("got non-nil detail on bare ctx: %+v", got)
	}
}

// TestWithAuthzResult_RoundTrip mirrors authn for authz.
func TestWithAuthzResult_RoundTrip(t *testing.T) {
	ctx := InstallHolder(context.Background())
	want := &auditpb.AuthzDetail{Relation: "admin", Decision: auditpb.AuthzDecision_AUTHZ_DECISION_ALLOWED}
	ctx = WithAuthzResult(ctx, want)

	got, ok := AuthzResultFrom(ctx)
	if !ok {
		t.Fatal("AuthzResultFrom returned ok=false")
	}
	if got != want {
		t.Errorf("got %p, want same pointer %p", got, want)
	}
}

// TestAuthzResultFrom_Empty returns (nil, false) on empty holder.
func TestAuthzResultFrom_Empty(t *testing.T) {
	ctx := InstallHolder(context.Background())
	got, ok := AuthzResultFrom(ctx)
	if ok {
		t.Errorf("ok=true on empty holder")
	}
	if got != nil {
		t.Errorf("got non-nil detail on empty holder: %+v", got)
	}
}

// TestAuthzResultFrom_NoHolder returns (nil, false) when no holder is installed.
func TestAuthzResultFrom_NoHolder(t *testing.T) {
	got, ok := AuthzResultFrom(context.Background())
	if ok {
		t.Errorf("ok=true on bare ctx")
	}
	if got != nil {
		t.Errorf("got non-nil detail on bare ctx: %+v", got)
	}
}

// TestWithAuthnResult_NilDetail returns the original ctx unchanged.
func TestWithAuthnResult_NilDetail(t *testing.T) {
	ctx := InstallHolder(context.Background())
	out := WithAuthnResult(ctx, nil)
	if got, ok := AuthnResultFrom(out); ok {
		t.Errorf("nil detail should not be stored; got %+v", got)
	}
}

// TestWithAuthzResult_NilDetail returns the original ctx unchanged.
func TestWithAuthzResult_NilDetail(t *testing.T) {
	ctx := InstallHolder(context.Background())
	out := WithAuthzResult(ctx, nil)
	if got, ok := AuthzResultFrom(out); ok {
		t.Errorf("nil detail should not be stored; got %+v", got)
	}
}

// TestWithAuthnResult_NoActiveSpan must not panic when ctx has no OTel span.
func TestWithAuthnResult_NoActiveSpan(t *testing.T) {
	ctx := InstallHolder(context.Background())
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic with no active span: %v", r)
		}
	}()
	_ = WithAuthnResult(ctx, &auditpb.AuthnDetail{Method: "jwt"})
}

// TestWithAuthnResult_AddsSpanEvent attaches an event to the active span.
// Uses local TracerProvider (not otel.SetTracerProvider) to avoid global state pollution.
func TestWithAuthnResult_AddsSpanEvent(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	ctx, span := tp.Tracer("test").Start(context.Background(), "test-span")
	ctx = InstallHolder(ctx)
	_ = WithAuthnResult(ctx, &auditpb.AuthnDetail{Method: "jwt"})
	span.End()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	events := spans[0].Events
	found := false
	for _, e := range events {
		if e.Name == "audit.authn.recorded" {
			found = true
		}
	}
	if !found {
		t.Errorf("audit.authn.recorded event not attached; got events: %v", events)
	}
}

// TestWithAuthzResult_AddsSpanEvent mirrors above for authz.
func TestWithAuthzResult_AddsSpanEvent(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	ctx, span := tp.Tracer("test").Start(context.Background(), "test-span")
	ctx = InstallHolder(ctx)
	_ = WithAuthzResult(ctx, &auditpb.AuthzDetail{Relation: "admin"})
	span.End()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	found := false
	for _, e := range spans[0].Events {
		if e.Name == "audit.authz.recorded" {
			found = true
		}
	}
	if !found {
		t.Errorf("audit.authz.recorded event not attached; got events: %v", spans[0].Events)
	}
}

// TestWithAuthnResult_NoHolder_Silent verifies the documented contract:
// when no holder is installed (Collector misconfigured or absent), writes are
// silently dropped, no panic, and no span event is fired.
func TestWithAuthnResult_NoHolder_Silent(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	ctx, span := tp.Tracer("test").Start(context.Background(), "test-span")
	// NB: no InstallHolder — this is the misconfigured path.
	out := WithAuthnResult(ctx, &auditpb.AuthnDetail{Method: "jwt", Success: true})
	span.End()

	if got, ok := AuthnResultFrom(out); ok {
		t.Errorf("write must be dropped without holder; got %+v", got)
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	for _, e := range spans[0].Events {
		if e.Name == "audit.authn.recorded" {
			t.Errorf("span event must not fire without holder; got events: %v", spans[0].Events)
		}
	}
}

// TestWithAuthzResult_NoHolder_Silent mirrors above for authz.
func TestWithAuthzResult_NoHolder_Silent(t *testing.T) {
	out := WithAuthzResult(context.Background(), &auditpb.AuthzDetail{Relation: "admin"})
	if got, ok := AuthzResultFrom(out); ok {
		t.Errorf("write must be dropped without holder; got %+v", got)
	}
}

// TestInstallHolder_Idempotent: calling InstallHolder twice keeps the first
// holder so existing writes aren't orphaned by a nested Collector.
func TestInstallHolder_Idempotent(t *testing.T) {
	ctx := InstallHolder(context.Background())
	d := &auditpb.AuthnDetail{Method: "jwt", Success: true}
	ctx = WithAuthnResult(ctx, d)

	// Second install must NOT replace the holder.
	ctx2 := InstallHolder(ctx)

	got, ok := AuthnResultFrom(ctx2)
	if !ok {
		t.Fatal("nested InstallHolder dropped earlier write")
	}
	if got != d {
		t.Errorf("nested InstallHolder swapped the holder; got %p want %p", got, d)
	}
}

// TestHolder_OuterReadsInnerWrite is the regression guard for the whole
// refactor's purpose: outer middleware installs holder, hands ctx to inner
// (a regular function suffices), inner mutates via WithAuthnResult, outer's
// ORIGINAL ctx now reads back the detail. With Go's immutable WithValue,
// this would fail; with the holder pointer, it succeeds.
func TestHolder_OuterReadsInnerWrite(t *testing.T) {
	outerCtx := InstallHolder(context.Background())

	// Simulate inner middleware: receives ctx, may even create children
	// internally, but the holder pointer survives in outerCtx.
	innerFn := func(ctx context.Context) {
		// The inner can do its own context.WithValue children — does not
		// matter, the holder lives on outerCtx and is reachable from any
		// descendant including ctx.
		child := context.WithValue(ctx, struct{ k string }{"unrelated"}, "x")
		_ = WithAuthnResult(child, &auditpb.AuthnDetail{Method: "jwt", Success: true})
	}
	innerFn(outerCtx)

	got, ok := AuthnResultFrom(outerCtx)
	if !ok {
		t.Fatal("outer ctx didn't see inner-middleware write — holder propagation broken")
	}
	if got.Method != "jwt" || !got.Success {
		t.Errorf("retrieved detail mismatch: %+v", got)
	}
}
