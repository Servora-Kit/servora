package audit

import (
	"context"
	"errors"
	"sync"
	"testing"

	auditv1 "github.com/Servora-Kit/servora/api/gen/go/servora/audit/v1"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/transport"
)

// mockTransport implements transport.Transporter for testing.
type mockTransport struct {
	operation string
}

func (m *mockTransport) Kind() transport.Kind            { return transport.KindGRPC }
func (m *mockTransport) Endpoint() string                { return "test-endpoint" }
func (m *mockTransport) Operation() string               { return m.operation }
func (m *mockTransport) RequestHeader() transport.Header { return nil }
func (m *mockTransport) ReplyHeader() transport.Header   { return nil }

// mockAuditor records emitted events.
type mockAuditor struct {
	mu     sync.Mutex
	events []cloudevents.Event
	err    error
}

func (m *mockAuditor) Emit(_ context.Context, event cloudevents.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
	return m.err
}

func (m *mockAuditor) Events() []cloudevents.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]cloudevents.Event{}, m.events...)
}

func testRules() map[string]*auditv1.AuditRule {
	return map[string]*auditv1.AuditRule{
		"/test.Service/Enabled": {
			Mode: auditv1.AuditMode_AUDIT_MODE_ENABLED,
		},
		"/test.Service/Disabled": {
			Mode: auditv1.AuditMode_AUDIT_MODE_DISABLED,
		},
	}
}

func TestMiddleware_EmitOnEnabledRule(t *testing.T) {
	ma := &mockAuditor{}
	mw := Middleware(ma, WithRulesFuncs(testRules))

	handler := mw(func(ctx context.Context, req interface{}) (interface{}, error) {
		return "response", nil
	})

	ctx := transport.NewServerContext(context.Background(), &mockTransport{
		operation: "/test.Service/Enabled",
	})

	resp, err := handler(ctx, "request")
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if resp != "response" {
		t.Errorf("resp = %v, want %q", resp, "response")
	}

	events := ma.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	e := events[0]
	// Middleware must emit a generic RPC audit event with the correct type and subject.
	if e.Type() != "servora.audit.rpc.v1" {
		t.Errorf("event Type = %q, want %q", e.Type(), "servora.audit.rpc.v1")
	}
	if e.Subject() != "/test.Service/Enabled" {
		t.Errorf("event Subject = %q, want %q", e.Subject(), "/test.Service/Enabled")
	}
}

func TestMiddleware_NoEmitOnDisabledRule(t *testing.T) {
	ma := &mockAuditor{}
	mw := Middleware(ma, WithRulesFuncs(testRules))

	handler := mw(func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	})

	ctx := transport.NewServerContext(context.Background(), &mockTransport{
		operation: "/test.Service/Disabled",
	})

	_, err := handler(ctx, nil)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	events := ma.Events()
	if len(events) != 0 {
		t.Errorf("expected 0 events for disabled rule, got %d", len(events))
	}
}

func TestMiddleware_NoEmitOnUnknownOperation(t *testing.T) {
	ma := &mockAuditor{}
	mw := Middleware(ma, WithRulesFuncs(testRules))

	handler := mw(func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	})

	ctx := transport.NewServerContext(context.Background(), &mockTransport{
		operation: "/test.Service/Unknown",
	})

	_, err := handler(ctx, nil)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	events := ma.Events()
	if len(events) != 0 {
		t.Errorf("expected 0 events for unknown operation, got %d", len(events))
	}
}

func TestMiddleware_EmitErrorDoesNotBlockHandler(t *testing.T) {
	ma := &mockAuditor{err: errors.New("emit failed")}
	mw := Middleware(ma, WithRulesFuncs(testRules))

	handler := mw(func(ctx context.Context, req interface{}) (interface{}, error) {
		return "success", nil
	})

	ctx := transport.NewServerContext(context.Background(), &mockTransport{
		operation: "/test.Service/Enabled",
	})

	resp, err := handler(ctx, nil)
	if err != nil {
		t.Errorf("handler should not fail even if emit fails: %v", err)
	}
	if resp != "success" {
		t.Errorf("resp = %v, want %q", resp, "success")
	}
}

func TestMiddleware_HandlerErrorPropagated(t *testing.T) {
	ma := &mockAuditor{}
	mw := Middleware(ma, WithRulesFuncs(testRules))

	handlerErr := errors.New("handler failed")
	handler := mw(func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, handlerErr
	})

	ctx := transport.NewServerContext(context.Background(), &mockTransport{
		operation: "/test.Service/Enabled",
	})

	_, err := handler(ctx, nil)
	if !errors.Is(err, handlerErr) {
		t.Errorf("expected handler error to be propagated, got: %v", err)
	}

	// Error should also be captured in the event.
	events := ma.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if v, ok := events[0].Extensions()["errormessage"]; !ok || v != "handler failed" {
		t.Errorf("errormessage = %v, want 'handler failed'", v)
	}
}

// Ensure Middleware satisfies the kratos middleware.Middleware signature.
var _ middleware.Middleware = Middleware(&mockAuditor{})
