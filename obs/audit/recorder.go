package audit

import (
	"context"
	"time"

	"github.com/Servora-Kit/servora/core/actor"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
)

// Recorder is the primary entrypoint for producing audit events.
// It wires actor extraction, auto-fills metadata, and delegates to an Emitter.
type Recorder struct {
	emitter     Emitter
	serviceName string
}

// NewRecorder creates a Recorder backed by the given Emitter.
func NewRecorder(emitter Emitter, serviceName string) *Recorder {
	if emitter == nil {
		emitter = NewNoopEmitter()
	}
	return &Recorder{emitter: emitter, serviceName: serviceName}
}

// Close releases the underlying emitter resources.
func (r *Recorder) Close() error { return r.emitter.Close() }

// RecordAuthzDecision records an OpenFGA authorization check result.
func (r *Recorder) RecordAuthzDecision(ctx context.Context, operation string, a actor.Actor, detail AuthzDetail) {
	evt := r.newEvent(ctx, EventTypeAuthzDecision, operation, a)
	evt.Target = TargetInfo{Type: detail.ObjectType, ID: detail.ObjectID}
	evt.Result = ResultInfo{Success: detail.Decision == AuthzDecisionAllowed}
	if detail.Decision == AuthzDecisionError {
		evt.Result.ErrorMessage = detail.ErrorReason
	}
	evt.Detail = detail
	_ = r.emitter.Emit(ctx, evt)
}

// RecordTupleChange records an OpenFGA tuple write or delete.
func (r *Recorder) RecordTupleChange(ctx context.Context, operation string, a actor.Actor, detail TupleMutationDetail) {
	evt := r.newEvent(ctx, EventTypeTupleChanged, operation, a)
	evt.Result = ResultInfo{Success: true}
	evt.Detail = detail
	if len(detail.Tuples) > 0 {
		evt.Target = TargetInfo{ID: detail.Tuples[0].Object}
	}
	_ = r.emitter.Emit(ctx, evt)
}

// RecordResourceMutation records a CRUD operation on a business resource.
func (r *Recorder) RecordResourceMutation(ctx context.Context, operation string, a actor.Actor, target TargetInfo, detail ResourceMutationDetail, err error) {
	evt := r.newEvent(ctx, EventTypeResourceMutation, operation, a)
	evt.Target = target
	evt.Detail = detail
	if err != nil {
		evt.Result = ResultInfo{Success: false, ErrorMessage: err.Error()}
	} else {
		evt.Result = ResultInfo{Success: true}
	}
	_ = r.emitter.Emit(ctx, evt)
}

// RecordAuthnResult records an authentication attempt result.
func (r *Recorder) RecordAuthnResult(ctx context.Context, operation string, a actor.Actor, detail AuthnDetail) {
	evt := r.newEvent(ctx, EventTypeAuthnResult, operation, a)
	evt.Detail = detail
	evt.Result = ResultInfo{Success: detail.Success}
	if !detail.Success {
		evt.Result.ErrorMessage = detail.FailureReason
	}
	_ = r.emitter.Emit(ctx, evt)
}

func (r *Recorder) newEvent(ctx context.Context, eventType EventType, operation string, a actor.Actor) *AuditEvent {
	evt := &AuditEvent{
		EventID:      uuid.NewString(),
		EventType:    eventType,
		EventVersion: "1.0",
		OccurredAt:   time.Now().UTC(),
		Service:      r.serviceName,
		Operation:    operation,
		TraceID:      traceIDFromContext(ctx),
		RequestID:    requestIDFromContext(ctx),
	}
	if a != nil {
		evt.Actor = ActorInfo{
			ID:          a.ID(),
			Type:        string(a.Type()),
			DisplayName: a.DisplayName(),
			Email:       a.Email(),
			Subject:     a.Subject(),
			ClientID:    a.ClientID(),
			Realm:       a.Realm(),
		}
	}
	return evt
}

func traceIDFromContext(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

func requestIDFromContext(ctx context.Context) string {
	tr, ok := transport.FromServerContext(ctx)
	if !ok {
		return ""
	}
	for _, key := range []string{"X-Request-ID", "X-Request-Id", "Request-Id"} {
		if value := tr.RequestHeader().Get(key); value != "" {
			return value
		}
	}
	return ""
}
