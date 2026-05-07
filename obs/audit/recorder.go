package audit

import (
	"context"
	"time"

	auditpb "github.com/Servora-Kit/servora/api/gen/go/servora/audit/v1"
	"github.com/Servora-Kit/servora/core/actor"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Recorder is the primary entrypoint for producing audit events.
// It wires actor extraction, auto-fills metadata, and delegates to an Emitter.
//
// 自 v0.4.4 起，所有 detail 参数与事件本体均使用 auditpb.* (proto 为 schema 单源)；
// 不再维护手写 runtime↔proto mapper。
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

// Emit sends a fully-built AuditEvent to the underlying emitter.
// 调用方（如 audit.Collector）若已组装好 *auditpb.AuditEvent，可直接 Emit；
// 错误不会冒泡到业务层（audit 失败不影响业务）。
func (r *Recorder) Emit(ctx context.Context, event *auditpb.AuditEvent) {
	if event == nil {
		return
	}
	_ = r.emitter.Emit(ctx, event)
}

// RecordAuthzDecision records an OpenFGA authorization check result.
func (r *Recorder) RecordAuthzDecision(ctx context.Context, operation string, a actor.Actor, detail *auditpb.AuthzDetail) {
	if detail == nil {
		return
	}
	evt := r.newEvent(ctx, auditpb.AuditEventType_AUDIT_EVENT_TYPE_AUTHZ_DECISION, operation, a)
	evt.Target = &auditpb.AuditTarget{Type: detail.GetObjectType(), Id: detail.GetObjectId()}
	evt.Result = &auditpb.AuditResult{Success: detail.GetDecision() == auditpb.AuthzDecision_AUTHZ_DECISION_ALLOWED}
	if detail.GetDecision() == auditpb.AuthzDecision_AUTHZ_DECISION_ERROR {
		evt.Result.ErrorMessage = detail.GetErrorReason()
	}
	evt.Detail = &auditpb.AuditEvent_AuthzDetail{AuthzDetail: detail}
	_ = r.emitter.Emit(ctx, evt)
}

// RecordTupleChange records an OpenFGA tuple write or delete.
func (r *Recorder) RecordTupleChange(ctx context.Context, operation string, a actor.Actor, detail *auditpb.TupleMutationDetail) {
	if detail == nil {
		return
	}
	evt := r.newEvent(ctx, auditpb.AuditEventType_AUDIT_EVENT_TYPE_TUPLE_CHANGED, operation, a)
	evt.Result = &auditpb.AuditResult{Success: true}
	evt.Detail = &auditpb.AuditEvent_TupleMutationDetail{TupleMutationDetail: detail}
	if tuples := detail.GetTuples(); len(tuples) > 0 {
		evt.Target = &auditpb.AuditTarget{Id: tuples[0].GetObject()}
	}
	_ = r.emitter.Emit(ctx, evt)
}

// RecordResourceMutation records a CRUD operation on a business resource.
func (r *Recorder) RecordResourceMutation(ctx context.Context, operation string, a actor.Actor, target *auditpb.AuditTarget, detail *auditpb.ResourceMutationDetail, err error) {
	evt := r.newEvent(ctx, auditpb.AuditEventType_AUDIT_EVENT_TYPE_RESOURCE_MUTATION, operation, a)
	if target != nil {
		evt.Target = target
	}
	if detail != nil {
		evt.Detail = &auditpb.AuditEvent_ResourceMutationDetail{ResourceMutationDetail: detail}
	}
	if err != nil {
		evt.Result = &auditpb.AuditResult{Success: false, ErrorMessage: err.Error()}
	} else {
		evt.Result = &auditpb.AuditResult{Success: true}
	}
	_ = r.emitter.Emit(ctx, evt)
}

// RecordAuthnResult records an authentication attempt result.
func (r *Recorder) RecordAuthnResult(ctx context.Context, operation string, a actor.Actor, detail *auditpb.AuthnDetail) {
	if detail == nil {
		return
	}
	evt := r.newEvent(ctx, auditpb.AuditEventType_AUDIT_EVENT_TYPE_AUTHN_RESULT, operation, a)
	evt.Detail = &auditpb.AuditEvent_AuthnDetail{AuthnDetail: detail}
	evt.Result = &auditpb.AuditResult{Success: detail.GetSuccess()}
	if !detail.GetSuccess() {
		evt.Result.ErrorMessage = detail.GetFailureReason()
	}
	_ = r.emitter.Emit(ctx, evt)
}

func (r *Recorder) newEvent(ctx context.Context, eventType auditpb.AuditEventType, operation string, a actor.Actor) *auditpb.AuditEvent {
	evt := &auditpb.AuditEvent{
		EventId:      uuid.NewString(),
		EventType:    eventType,
		EventVersion: "1.0",
		OccurredAt:   timestamppb.New(time.Now().UTC()),
		Service:      r.serviceName,
		Operation:    operation,
		TraceId:      traceIDFromContext(ctx),
		RequestId:    requestIDFromContext(ctx),
	}
	if a != nil {
		evt.Actor = &auditpb.AuditActor{
			Id:          a.ID(),
			Type:        string(a.Type()),
			DisplayName: a.DisplayName(),
			Email:       a.Email(),
			Subject:     a.Subject(),
			ClientId:    a.ClientID(),
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
