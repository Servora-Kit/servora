package audit

import (
	"context"
	"errors"
	"testing"

	"github.com/Servora-Kit/servora/core/actor"
	"github.com/go-kratos/kratos/v2/transport"
)

type stubAuditTransport struct {
	op string
}

func (s *stubAuditTransport) Kind() transport.Kind             { return transport.KindHTTP }
func (s *stubAuditTransport) Endpoint() string                 { return "" }
func (s *stubAuditTransport) Operation() string                { return s.op }
func (s *stubAuditTransport) RequestHeader() transport.Header  { return &stubAuditHeader{} }
func (s *stubAuditTransport) ReplyHeader() transport.Header    { return &stubAuditHeader{} }

type stubAuditHeader struct{}

func (h *stubAuditHeader) Get(string) string      { return "" }
func (h *stubAuditHeader) Set(string, string)     {}
func (h *stubAuditHeader) Add(string, string)     {}
func (h *stubAuditHeader) Keys() []string         { return nil }
func (h *stubAuditHeader) Values(string) []string { return nil }

type captureEmitter struct {
	events []*AuditEvent
}

func (e *captureEmitter) Emit(_ context.Context, event *AuditEvent) error {
	e.events = append(e.events, event)
	return nil
}

func (e *captureEmitter) Close() error { return nil }

func newAuditTestContext(op string) context.Context {
	ctx := actor.NewContext(context.Background(), actor.NewUserActor(actor.UserActorParams{ID: "u1"}))
	return transport.NewServerContext(ctx, &stubAuditTransport{op: op})
}

func TestAuditMiddleware_RecordsResourceMutation(t *testing.T) {
	emitter := &captureEmitter{}
	recorder := NewRecorder(emitter, "iam")
	mw := Audit(
		WithRecorder(recorder),
		WithRules(map[string]Rule{
			"/audit.v1.Test/Mutate": {
				EventType: EventTypeResourceMutation,
				TargetType: "user",
			},
		}),
	)

	handler := mw(func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	})

	_, err := handler(newAuditTestContext("/audit.v1.Test/Mutate"), nil)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if len(emitter.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(emitter.events))
	}
	if emitter.events[0].Target.Type != "user" {
		t.Fatalf("target type = %q, want user", emitter.events[0].Target.Type)
	}
	detail, ok := emitter.events[0].Detail.(ResourceMutationDetail)
	if !ok {
		t.Fatalf("expected ResourceMutationDetail, got %T", emitter.events[0].Detail)
	}
	if detail.ResourceType != "user" {
		t.Fatalf("resource type = %q, want user", detail.ResourceType)
	}
}

func TestAuditMiddleware_PassesThroughWhenRuleMissing(t *testing.T) {
	emitter := &captureEmitter{}
	recorder := NewRecorder(emitter, "iam")
	mw := Audit(WithRecorder(recorder), WithRules(map[string]Rule{}))

	handler := mw(func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	})

	resp, err := handler(newAuditTestContext("/audit.v1.Test/Missing"), nil)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if resp != "ok" {
		t.Fatalf("response = %v, want ok", resp)
	}
	if len(emitter.events) != 0 {
		t.Fatalf("expected 0 audit events, got %d", len(emitter.events))
	}
}

func TestAuditMiddleware_DoesNotRecordErrorWhenDisabled(t *testing.T) {
	emitter := &captureEmitter{}
	recorder := NewRecorder(emitter, "iam")
	mw := Audit(
		WithRecorder(recorder),
		WithRules(map[string]Rule{
			"/audit.v1.Test/Mutate": {
				EventType:     EventTypeResourceMutation,
				TargetType:    "user",
				RecordOnError: false,
			},
		}),
	)

	expectedErr := errors.New("boom")
	handler := mw(func(ctx context.Context, req any) (any, error) {
		return nil, expectedErr
	})

	_, err := handler(newAuditTestContext("/audit.v1.Test/Mutate"), nil)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("handler error = %v, want %v", err, expectedErr)
	}
	if len(emitter.events) != 0 {
		t.Fatalf("expected 0 audit events, got %d", len(emitter.events))
	}
}

func TestAuditMiddleware_RecordsErrorWhenEnabled(t *testing.T) {
	emitter := &captureEmitter{}
	recorder := NewRecorder(emitter, "iam")
	mw := Audit(
		WithRecorder(recorder),
		WithRules(map[string]Rule{
			"/audit.v1.Test/Mutate": {
				EventType:     EventTypeResourceMutation,
				TargetType:    "user",
				RecordOnError: true,
			},
		}),
	)

	expectedErr := errors.New("boom")
	handler := mw(func(ctx context.Context, req any) (any, error) {
		return nil, expectedErr
	})

	_, err := handler(newAuditTestContext("/audit.v1.Test/Mutate"), nil)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("handler error = %v, want %v", err, expectedErr)
	}
	if len(emitter.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(emitter.events))
	}
	if emitter.events[0].Result.Success {
		t.Fatal("expected recorded event to be unsuccessful")
	}
	if emitter.events[0].Result.ErrorMessage != expectedErr.Error() {
		t.Fatalf("error message = %q, want %q", emitter.events[0].Result.ErrorMessage, expectedErr.Error())
	}
}

func TestAuditMiddleware_DoesNotRecordUnsupportedEventType(t *testing.T) {
	emitter := &captureEmitter{}
	recorder := NewRecorder(emitter, "iam")
	mw := Audit(
		WithRecorder(recorder),
		WithRules(map[string]Rule{
			"/audit.v1.Test/Mutate": {
				EventType: EventTypeAuthzDecision,
				TargetType: "user",
			},
		}),
	)

	handler := mw(func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	})

	_, err := handler(newAuditTestContext("/audit.v1.Test/Mutate"), nil)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if len(emitter.events) != 0 {
		t.Fatalf("expected 0 audit events for unsupported event type, got %d", len(emitter.events))
	}
}
