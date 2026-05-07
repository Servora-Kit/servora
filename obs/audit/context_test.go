package audit

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	auditpb "github.com/Servora-Kit/servora/api/gen/go/servora/audit/v1"
)

// TestWithAuthnResult_RoundTrip checks that detail written to ctx can be retrieved.
func TestWithAuthnResult_RoundTrip(t *testing.T) {
	ctx := context.Background()
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

// TestAuthnResultFrom_Empty returns (nil, false) when no detail was written.
func TestAuthnResultFrom_Empty(t *testing.T) {
	got, ok := AuthnResultFrom(context.Background())
	if ok {
		t.Errorf("ok=true on empty ctx, got %+v", got)
	}
	if got != nil {
		t.Errorf("got non-nil detail on empty ctx: %+v", got)
	}
}

// TestWithAuthzResult_RoundTrip mirrors authn for authz.
func TestWithAuthzResult_RoundTrip(t *testing.T) {
	ctx := context.Background()
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

// TestAuthzResultFrom_Empty returns (nil, false) on empty ctx.
func TestAuthzResultFrom_Empty(t *testing.T) {
	got, ok := AuthzResultFrom(context.Background())
	if ok || got != nil {
		t.Errorf("expected (nil,false), got (%+v,%v)", got, ok)
	}
}

// TestWithAuthnResult_NoActiveSpan must not panic when ctx has no OTel span.
func TestWithAuthnResult_NoActiveSpan(t *testing.T) {
	ctx := context.Background()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic with no active span: %v", r)
		}
	}()
	_ = WithAuthnResult(ctx, &auditpb.AuthnDetail{Method: "jwt"})
}

// TestWithAuthnResult_AddsSpanEvent attaches an event to the active span.
func TestWithAuthnResult_AddsSpanEvent(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	ctx, span := tp.Tracer("test").Start(context.Background(), "test-span")
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
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	ctx, span := tp.Tracer("test").Start(context.Background(), "test-span")
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
