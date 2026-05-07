package audit

import (
	"context"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"go.opentelemetry.io/otel/trace"

	auditpb "github.com/Servora-Kit/servora/api/gen/go/servora/audit/v1"
	"github.com/Servora-Kit/servora/core/actor"
)

// CollectorOption configures Collector behaviour. Named with the Collector
// prefix for parallelism with AuditMiddlewareOption (middleware.go).
type CollectorOption func(*collectorConfig)

type collectorConfig struct {
	spanEvents bool
}

// WithSpanEvents toggles the trailing "audit.collected" span event emitted by
// Collector after it processes ctx detail. Default is true. Disabling this
// does NOT affect the write-time span events from WithAuthnResult /
// WithAuthzResult — those fire upstream before Collector ever runs.
func WithSpanEvents(enabled bool) CollectorOption {
	return func(c *collectorConfig) { c.spanEvents = enabled }
}

// Collector is a transport middleware that runs at the chain tail. After the
// inner handler returns, it reads *auditpb.AuthnDetail / *auditpb.AuthzDetail
// from ctx (written by security/authn / security/authz middleware via
// WithAuthnResult / WithAuthzResult) and emits one *auditpb.AuditEvent per
// detail present.
//
// Mounting position: Collector MUST be mounted AFTER authn/authz middleware so
// the details are present in ctx by the time the post-handler phase runs.
// Mounting earlier degrades silently to "no events emitted" — see
// audit-context-collector spec, requirement 3.
//
// Result semantics: each emitted AuditEvent.Result reflects ONLY its own
// layer's outcome. AUTHN_RESULT.Result mirrors AuthnDetail.Success /
// FailureReason; AUTHZ_DECISION.Result mirrors Decision / ErrorReason. The
// handler's business error is propagated via the middleware return value but
// never leaks into AuditEvent.Result — RPC-layer errors are recorded by
// RESOURCE_MUTATION events (proto-annotation driven), not by authn/authz
// audit records. This lets consumers distinguish "authn failed" from
// "authn ok but business failed" by EventType alone.
func Collector(rec *Recorder, opts ...CollectorOption) middleware.Middleware {
	if rec == nil {
		// Defensive default: mirrors Audit middleware (middleware.go) and
		// keeps Collector(nil) from nil-derefing in misconfigured wireup.
		rec = NewRecorder(nil, "")
	}
	cfg := &collectorConfig{spanEvents: true}
	for _, opt := range opts {
		opt(cfg)
	}

	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			resp, handlerErr := handler(ctx, req)

			emitFromContext(ctx, rec)

			if cfg.spanEvents {
				trace.SpanFromContext(ctx).AddEvent(spanEventCollected)
			}

			return resp, handlerErr
		}
	}
}

// emitFromContext inspects ctx for authn/authz detail and emits one event per
// detail present. Emission order: authn → authz (matching spec scenario
// "Collector 同时 emit authn + authz 事件").
func emitFromContext(ctx context.Context, rec *Recorder) {
	operation := operationFromContext(ctx)
	a, _ := actor.FromContext(ctx)

	// Emit errors are intentionally silent at this layer — Recorder.Emit's contract
	// is fire-and-forget for audit (audit failures must not break business flow).
	// If observability of emit failures is needed, instrument it inside Emitter
	// implementations.
	if d, ok := AuthnResultFrom(ctx); ok {
		_ = rec.Emit(ctx, buildAuthnEvent(ctx, rec, operation, a, d))
	}
	if d, ok := AuthzResultFrom(ctx); ok {
		_ = rec.Emit(ctx, buildAuthzEvent(ctx, rec, operation, a, d))
	}
}

// buildAuthnEvent assembles an AUTHN_RESULT AuditEvent. Result reflects the
// authn layer outcome only (matches RecordAuthnResult precedent).
func buildAuthnEvent(ctx context.Context, rec *Recorder, operation string, a actor.Actor, d *auditpb.AuthnDetail) *auditpb.AuditEvent {
	evt := rec.buildEvent(ctx, auditpb.AuditEventType_AUDIT_EVENT_TYPE_AUTHN_RESULT, operation, a)
	evt.Detail = &auditpb.AuditEvent_AuthnDetail{AuthnDetail: d}
	evt.Result = &auditpb.AuditResult{Success: d.GetSuccess()}
	if !d.GetSuccess() {
		evt.Result.ErrorMessage = d.GetFailureReason()
	}
	return evt
}

// buildAuthzEvent assembles an AUTHZ_DECISION AuditEvent. Target is populated
// from detail's ObjectType/ObjectId; Result reflects the authz layer outcome
// only (matches RecordAuthzDecision precedent).
func buildAuthzEvent(ctx context.Context, rec *Recorder, operation string, a actor.Actor, d *auditpb.AuthzDetail) *auditpb.AuditEvent {
	evt := rec.buildEvent(ctx, auditpb.AuditEventType_AUDIT_EVENT_TYPE_AUTHZ_DECISION, operation, a)
	evt.Target = &auditpb.AuditTarget{Type: d.GetObjectType(), Id: d.GetObjectId()}
	evt.Detail = &auditpb.AuditEvent_AuthzDetail{AuthzDetail: d}
	evt.Result = &auditpb.AuditResult{Success: d.GetDecision() == auditpb.AuthzDecision_AUTHZ_DECISION_ALLOWED}
	if d.GetDecision() == auditpb.AuthzDecision_AUTHZ_DECISION_ERROR {
		evt.Result.ErrorMessage = d.GetErrorReason()
	}
	return evt
}

// operationFromContext extracts the transport operation path. Returns "" when
// no transport ServerContext is present (e.g. unit tests calling collector
// directly without Kratos transport).
func operationFromContext(ctx context.Context) string {
	if tr, ok := transport.FromServerContext(ctx); ok {
		return tr.Operation()
	}
	return ""
}
