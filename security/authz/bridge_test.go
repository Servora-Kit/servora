package authz

import (
	"context"
	"testing"

	"github.com/Servora-Kit/servora/obs/audit"
)

// captureEmitter records every event emitted, for assertion.
type captureEmitter struct {
	events []*audit.AuditEvent
}

func (c *captureEmitter) Emit(_ context.Context, e *audit.AuditEvent) error {
	c.events = append(c.events, e)
	return nil
}
func (c *captureEmitter) Close() error { return nil }

// TestNewAuthzBridge_AllowedDecisionRecordsAuthzEvent verifies wire-up.
func TestNewAuthzBridge_AllowedDecisionRecordsAuthzEvent(t *testing.T) {
	emitter := &captureEmitter{}
	recorder := audit.NewRecorder(emitter, "test-svc")
	bridge := NewAuthzBridge(recorder)

	ctx := userActorCtx(transportCtx(testOp), "user-123")
	bridge(ctx, DecisionDetail{
		Operation:  testOp,
		Subject:    "user:user-123",
		Relation:   "viewer",
		ObjectType: "doc",
		ObjectID:   "doc-1",
		Allowed:    true,
	})

	if len(emitter.events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(emitter.events))
	}
	evt := emitter.events[0]
	if evt.EventType != audit.EventTypeAuthzDecision {
		t.Errorf("evt.EventType = %s, want %s", evt.EventType, audit.EventTypeAuthzDecision)
	}
	if evt.Operation != testOp {
		t.Errorf("evt.Operation = %s, want %s", evt.Operation, testOp)
	}
	detail, ok := evt.Detail.(audit.AuthzDetail)
	if !ok {
		t.Fatalf("evt.Detail = %T, want audit.AuthzDetail", evt.Detail)
	}
	if detail.Decision != audit.AuthzDecisionAllowed {
		t.Errorf("detail.Decision = %s, want allowed", detail.Decision)
	}
	if detail.ObjectID != "doc-1" {
		t.Errorf("detail.ObjectID = %s, want doc-1", detail.ObjectID)
	}
}

// TestNewAuthzBridge_DeniedDecisionMapsToDenied checks decision mapping.
func TestNewAuthzBridge_DeniedDecisionMapsToDenied(t *testing.T) {
	emitter := &captureEmitter{}
	recorder := audit.NewRecorder(emitter, "test-svc")
	bridge := NewAuthzBridge(recorder)

	ctx := userActorCtx(transportCtx(testOp), "user-123")
	bridge(ctx, DecisionDetail{
		Operation: testOp, Subject: "user:user-123", Relation: "admin",
		ObjectType: "platform", ObjectID: "default", Allowed: false,
	})

	if len(emitter.events) != 1 {
		t.Fatalf("len(events) = %d", len(emitter.events))
	}
	detail := emitter.events[0].Detail.(audit.AuthzDetail)
	if detail.Decision != audit.AuthzDecisionDenied {
		t.Errorf("detail.Decision = %s, want denied", detail.Decision)
	}
}

// TestNewAuthzBridge_ErrorMapsToErrorDecision uses Err to mean engine failure.
func TestNewAuthzBridge_ErrorMapsToErrorDecision(t *testing.T) {
	emitter := &captureEmitter{}
	recorder := audit.NewRecorder(emitter, "test-svc")
	bridge := NewAuthzBridge(recorder)

	ctx := userActorCtx(transportCtx(testOp), "user-123")
	bridge(ctx, DecisionDetail{
		Operation: testOp, Subject: "user:user-123",
		Relation: "x", ObjectType: "y", ObjectID: "z",
		Err: context.DeadlineExceeded,
	})

	detail := emitter.events[0].Detail.(audit.AuthzDetail)
	if detail.Decision != audit.AuthzDecisionError {
		t.Errorf("detail.Decision = %s, want error", detail.Decision)
	}
	if detail.ErrorReason == "" {
		t.Errorf("detail.ErrorReason should be populated on Err")
	}
}

// TestNewAuthzBridge_NilRecorder_NoOp ensures safe wiring with nil.
func TestNewAuthzBridge_NilRecorder_NoOp(t *testing.T) {
	bridge := NewAuthzBridge(nil)
	// Must not panic.
	bridge(context.Background(), DecisionDetail{Operation: testOp, Allowed: true})
}
