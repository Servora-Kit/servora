package audit

import (
	"context"
	"errors"
	"testing"

	auditpb "github.com/Servora-Kit/servora/api/gen/go/servora/audit/v1"
	"github.com/Servora-Kit/servora/core/actor"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// newCollectorTestContext returns a ctx with a transport ServerContext (op set)
// and an actor — mirroring the shape Collector sees in production.
func newCollectorTestContext(op string) context.Context {
	ctx := actor.NewContext(context.Background(), actor.NewUserActor(actor.UserActorParams{ID: "u1"}))
	return transport.NewServerContext(ctx, &stubAuditTransport{op: op})
}

func newPassthroughHandler(resp any, err error) middleware.Handler {
	return func(ctx context.Context, req any) (any, error) { return resp, err }
}

// inner builds an inner-handler that simulates security middleware: it calls
// the supplied write fn against the ctx (after Collector installs holder),
// then returns resp/err. This matches production: Collector is outer, writes
// happen below it.
func inner(write func(ctx context.Context), resp any, err error) middleware.Handler {
	return func(ctx context.Context, req any) (any, error) {
		if write != nil {
			write(ctx)
		}
		return resp, err
	}
}

// TestCollector_EmitsAuthnOnly covers spec scenario:
// "Collector 仅 emit ctx 中存在 detail 类型的事件" — authn detail only, no authz.
func TestCollector_EmitsAuthnOnly(t *testing.T) {
	emitter := &captureEmitter{}
	rec := NewRecorder(emitter, "test-svc")

	mw := Collector(rec)
	handler := mw(inner(func(ctx context.Context) {
		_ = WithAuthnResult(ctx, &auditpb.AuthnDetail{Method: "jwt", Success: true})
	}, "ok", nil))

	_, err := handler(newCollectorTestContext("/test/Op"), nil)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if len(emitter.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(emitter.events))
	}
	if emitter.events[0].GetEventType() != auditpb.AuditEventType_AUDIT_EVENT_TYPE_AUTHN_RESULT {
		t.Errorf("event type = %v, want AUTHN_RESULT", emitter.events[0].GetEventType())
	}
	if emitter.events[0].GetAuthnDetail().GetMethod() != "jwt" {
		t.Errorf("method = %q, want jwt", emitter.events[0].GetAuthnDetail().GetMethod())
	}
	if !emitter.events[0].GetResult().GetSuccess() {
		t.Errorf("Result.Success = false, want true (authn ok + handler ok)")
	}
}

// TestCollector_EmitsAuthzOnly: only authz detail in ctx → 1 AUTHZ_DECISION event.
func TestCollector_EmitsAuthzOnly(t *testing.T) {
	emitter := &captureEmitter{}
	rec := NewRecorder(emitter, "test-svc")

	mw := Collector(rec)
	handler := mw(inner(func(ctx context.Context) {
		_ = WithAuthzResult(ctx, &auditpb.AuthzDetail{
			Relation:   "owner",
			ObjectType: "doc",
			ObjectId:   "d1",
			Decision:   auditpb.AuthzDecision_AUTHZ_DECISION_ALLOWED,
		})
	}, "ok", nil))

	_, err := handler(newCollectorTestContext("/test/Op"), nil)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if len(emitter.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(emitter.events))
	}
	if emitter.events[0].GetEventType() != auditpb.AuditEventType_AUDIT_EVENT_TYPE_AUTHZ_DECISION {
		t.Errorf("event type = %v, want AUTHZ_DECISION", emitter.events[0].GetEventType())
	}
	if emitter.events[0].GetTarget().GetType() != "doc" || emitter.events[0].GetTarget().GetId() != "d1" {
		t.Errorf("target mismatch: %+v", emitter.events[0].GetTarget())
	}
	if !emitter.events[0].GetResult().GetSuccess() {
		t.Errorf("Result.Success = false, want true (allowed + handler ok)")
	}
}

// TestCollector_EmitsBoth covers spec scenario:
// "Collector 同时 emit authn + authz 事件" — both details written, two events emitted in order.
func TestCollector_EmitsBoth(t *testing.T) {
	emitter := &captureEmitter{}
	rec := NewRecorder(emitter, "test-svc")

	mw := Collector(rec)
	handler := mw(inner(func(ctx context.Context) {
		_ = WithAuthnResult(ctx, &auditpb.AuthnDetail{Method: "jwt", Success: true})
		_ = WithAuthzResult(ctx, &auditpb.AuthzDetail{
			Relation: "viewer",
			Decision: auditpb.AuthzDecision_AUTHZ_DECISION_ALLOWED,
		})
	}, "ok", nil))

	_, err := handler(newCollectorTestContext("/test/Op"), nil)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if len(emitter.events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(emitter.events))
	}
	if emitter.events[0].GetEventType() != auditpb.AuditEventType_AUDIT_EVENT_TYPE_AUTHN_RESULT {
		t.Errorf("event[0] type = %v, want AUTHN_RESULT", emitter.events[0].GetEventType())
	}
	if emitter.events[1].GetEventType() != auditpb.AuditEventType_AUDIT_EVENT_TYPE_AUTHZ_DECISION {
		t.Errorf("event[1] type = %v, want AUTHZ_DECISION", emitter.events[1].GetEventType())
	}
}

// TestCollector_SilentWhenNoDetail covers spec scenario:
// "Collector 在 ctx 无任何 detail 时静默" — no details written → 0 events.
func TestCollector_SilentWhenNoDetail(t *testing.T) {
	emitter := &captureEmitter{}
	rec := NewRecorder(emitter, "test-svc")

	mw := Collector(rec)
	handler := mw(newPassthroughHandler("ok", nil))

	resp, err := handler(newCollectorTestContext("/test/Op"), nil)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if resp != "ok" {
		t.Errorf("resp = %v, want ok", resp)
	}
	if len(emitter.events) != 0 {
		t.Errorf("expected 0 events, got %d", len(emitter.events))
	}
}

// TestCollector_EmitsAfterHandlerError covers spec scenario:
// "Collector 在 handler 返回后 emit，但 Result 仅反映 detail" — handler returns error,
// event still emitted; Result reflects ONLY the authn layer outcome (not the handler
// business error). The handler error propagates via the middleware return value but
// must not leak into AuditEvent.Result. Handler RPC-layer errors are recorded by
// RESOURCE_MUTATION events independently.
func TestCollector_EmitsAfterHandlerError(t *testing.T) {
	emitter := &captureEmitter{}
	rec := NewRecorder(emitter, "test-svc")

	handlerErr := errors.New("downstream failed")
	mw := Collector(rec)
	handler := mw(inner(func(ctx context.Context) {
		_ = WithAuthnResult(ctx, &auditpb.AuthnDetail{Method: "jwt", Success: true})
	}, nil, handlerErr))

	_, err := handler(newCollectorTestContext("/test/Op"), nil)
	if !errors.Is(err, handlerErr) {
		t.Fatalf("handler error not propagated: got %v want %v", err, handlerErr)
	}
	if len(emitter.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(emitter.events))
	}
	if !emitter.events[0].GetResult().GetSuccess() {
		t.Errorf("Result.Success = false, want true (authn ok; handler err must NOT leak into Result)")
	}
	if emitter.events[0].GetResult().GetErrorMessage() != "" {
		t.Errorf("ErrorMessage = %q, want empty (handler err must NOT leak into Result)", emitter.events[0].GetResult().GetErrorMessage())
	}
}

// TestCollector_AuthnFailedAndHandlerErrored verifies that when BOTH authn fails
// AND handler returns an error, Result still reflects ONLY the authn outcome
// (FailureReason from authn, not handler err). This proves the handler error
// channel and the AuditEvent.Result channel are fully independent.
func TestCollector_AuthnFailedAndHandlerErrored(t *testing.T) {
	emitter := &captureEmitter{}
	rec := NewRecorder(emitter, "test-svc")

	handlerErr := errors.New("downstream failed")
	mw := Collector(rec)
	handler := mw(inner(func(ctx context.Context) {
		_ = WithAuthnResult(ctx, &auditpb.AuthnDetail{
			Method:        "jwt",
			Success:       false,
			FailureReason: "token expired",
		})
	}, nil, handlerErr))

	_, err := handler(newCollectorTestContext("/test/Op"), nil)
	if !errors.Is(err, handlerErr) {
		t.Fatalf("handler error not propagated: got %v want %v", err, handlerErr)
	}
	if len(emitter.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(emitter.events))
	}
	if emitter.events[0].GetResult().GetSuccess() {
		t.Errorf("Result.Success = true, want false (authn failed)")
	}
	if emitter.events[0].GetResult().GetErrorMessage() != "token expired" {
		t.Errorf("ErrorMessage = %q, want %q (must come from authn FailureReason, not handler err)",
			emitter.events[0].GetResult().GetErrorMessage(), "token expired")
	}
}

// TestCollector_AuthnFailureReason: authn detail Success=false → Result.Success=false +
// FailureReason copied to ErrorMessage.
func TestCollector_AuthnFailureReason(t *testing.T) {
	emitter := &captureEmitter{}
	rec := NewRecorder(emitter, "test-svc")

	mw := Collector(rec)
	handler := mw(inner(func(ctx context.Context) {
		_ = WithAuthnResult(ctx, &auditpb.AuthnDetail{
			Method:        "jwt",
			Success:       false,
			FailureReason: "token expired",
		})
	}, "ok", nil))

	_, err := handler(newCollectorTestContext("/test/Op"), nil)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if len(emitter.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(emitter.events))
	}
	if emitter.events[0].GetResult().GetSuccess() {
		t.Errorf("Result.Success = true, want false (authn failed)")
	}
	if emitter.events[0].GetResult().GetErrorMessage() != "token expired" {
		t.Errorf("ErrorMessage = %q, want token expired", emitter.events[0].GetResult().GetErrorMessage())
	}
}

// TestCollector_AuthzErrorDecision: authz Decision=ERROR → Result.Success=false +
// ErrorReason copied to ErrorMessage.
func TestCollector_AuthzErrorDecision(t *testing.T) {
	emitter := &captureEmitter{}
	rec := NewRecorder(emitter, "test-svc")

	mw := Collector(rec)
	handler := mw(inner(func(ctx context.Context) {
		_ = WithAuthzResult(ctx, &auditpb.AuthzDetail{
			Relation:    "owner",
			Decision:    auditpb.AuthzDecision_AUTHZ_DECISION_ERROR,
			ErrorReason: "fga unreachable",
		})
	}, "ok", nil))

	_, err := handler(newCollectorTestContext("/test/Op"), nil)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if len(emitter.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(emitter.events))
	}
	if emitter.events[0].GetResult().GetSuccess() {
		t.Errorf("Result.Success = true, want false (authz error)")
	}
	if emitter.events[0].GetResult().GetErrorMessage() != "fga unreachable" {
		t.Errorf("ErrorMessage = %q, want fga unreachable", emitter.events[0].GetResult().GetErrorMessage())
	}
}

// TestCollector_SpanEventDefault: by default, Collector adds an "audit.collected"
// span event after emission.
func TestCollector_SpanEventDefault(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	emitter := &captureEmitter{}
	rec := NewRecorder(emitter, "test-svc")

	mw := Collector(rec)
	handler := mw(inner(func(ctx context.Context) {
		_ = WithAuthnResult(ctx, &auditpb.AuthnDetail{Method: "jwt", Success: true})
	}, "ok", nil))

	baseCtx := newCollectorTestContext("/test/Op")
	ctx, span := tp.Tracer("test").Start(baseCtx, "test-span")
	_, err := handler(ctx, nil)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	span.End()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	found := false
	for _, e := range spans[0].Events {
		if e.Name == "audit.collected" {
			found = true
		}
	}
	if !found {
		t.Errorf("audit.collected event missing; got events: %v", spans[0].Events)
	}
}

// TestCollector_SpanEventDisabled covers spec scenario:
// "WithSpanEvents(false) 关闭 collector 自身 span event" — only audit.collected is suppressed,
// authn/authz span events from WithAuthnResult are unaffected.
func TestCollector_SpanEventDisabled(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	emitter := &captureEmitter{}
	rec := NewRecorder(emitter, "test-svc")

	mw := Collector(rec, WithSpanEvents(false))
	handler := mw(inner(func(ctx context.Context) {
		_ = WithAuthnResult(ctx, &auditpb.AuthnDetail{Method: "jwt", Success: true})
	}, "ok", nil))

	baseCtx := newCollectorTestContext("/test/Op")
	ctx, span := tp.Tracer("test").Start(baseCtx, "test-span")
	_, err := handler(ctx, nil)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	span.End()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	for _, e := range spans[0].Events {
		if e.Name == "audit.collected" {
			t.Errorf("audit.collected event should be suppressed; got events: %v", spans[0].Events)
		}
	}
	// authn write-time event still fires.
	authnEventFound := false
	for _, e := range spans[0].Events {
		if e.Name == "audit.authn.recorded" {
			authnEventFound = true
		}
	}
	if !authnEventFound {
		t.Errorf("audit.authn.recorded should still fire (only collector's event is disabled)")
	}
}

// TestCollector_NoTransportContext: ensure Collector handles ctx without a transport
// ServerContext gracefully (operation falls back to empty string).
func TestCollector_NoTransportContext(t *testing.T) {
	emitter := &captureEmitter{}
	rec := NewRecorder(emitter, "test-svc")

	mw := Collector(rec)
	handler := mw(inner(func(ctx context.Context) {
		_ = WithAuthnResult(ctx, &auditpb.AuthnDetail{Method: "jwt", Success: true})
	}, "ok", nil))

	_, err := handler(context.Background(), nil)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if len(emitter.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(emitter.events))
	}
}
