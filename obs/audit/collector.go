package audit

import (
	"context"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"go.opentelemetry.io/otel/trace"

	auditpb "github.com/Servora-Kit/servora/api/gen/go/servora/audit/v1"
	"github.com/Servora-Kit/servora/core/actor"
)

// spanEventCollected is attached by Collector after emission to mark the
// pipeline tail on the active OTel span. Pairs with audit.authn.recorded /
// audit.authz.recorded set by WithAuthnResult / WithAuthzResult.
const spanEventCollected = "audit.collected"

// Option configures Collector behaviour.
type Option func(*collectorConfig)

type collectorConfig struct {
	spanEvents bool
}

// WithSpanEvents toggles the trailing "audit.collected" span event emitted by
// Collector after it processes ctx detail. Default is true. Disabling this
// does NOT affect the write-time span events from WithAuthnResult /
// WithAuthzResult — those fire upstream before Collector ever runs.
func WithSpanEvents(enabled bool) Option {
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
// Result.Success / Result.ErrorMessage reflect the handler outcome when the
// handler returns a non-nil error (per spec scenario "Collector 在 handler
// 返回后 emit"). When the handler succeeds, success is derived from the
// authn/authz detail itself (Success flag for authn, Decision==ALLOWED for authz).
func Collector(rec *Recorder, opts ...Option) middleware.Middleware {
	cfg := &collectorConfig{spanEvents: true}
	for _, opt := range opts {
		opt(cfg)
	}

	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			resp, handlerErr := handler(ctx, req)

			emitFromContext(ctx, rec, handlerErr)

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
func emitFromContext(ctx context.Context, rec *Recorder, handlerErr error) {
	operation := operationFromContext(ctx)
	a, _ := actor.FromContext(ctx)

	if d, ok := AuthnResultFrom(ctx); ok {
		_ = rec.Emit(ctx, buildAuthnEvent(rec, ctx, operation, a, d, handlerErr))
	}
	if d, ok := AuthzResultFrom(ctx); ok {
		_ = rec.Emit(ctx, buildAuthzEvent(rec, ctx, operation, a, d, handlerErr))
	}
}

// buildAuthnEvent assembles an AUTHN_RESULT AuditEvent. When handlerErr is
// non-nil, it overrides Result (handler outcome takes precedence per spec).
// Otherwise Result mirrors detail.Success / FailureReason (matches
// RecordAuthnResult precedent).
func buildAuthnEvent(rec *Recorder, ctx context.Context, operation string, a actor.Actor, d *auditpb.AuthnDetail, handlerErr error) *auditpb.AuditEvent {
	evt := rec.buildEvent(ctx, auditpb.AuditEventType_AUDIT_EVENT_TYPE_AUTHN_RESULT, operation, a)
	evt.Detail = &auditpb.AuditEvent_AuthnDetail{AuthnDetail: d}
	evt.Result = resultForDetail(d.GetSuccess(), d.GetFailureReason(), handlerErr)
	return evt
}

// buildAuthzEvent assembles an AUTHZ_DECISION AuditEvent. Target is populated
// from detail's ObjectType/ObjectId (matching RecordAuthzDecision precedent).
func buildAuthzEvent(rec *Recorder, ctx context.Context, operation string, a actor.Actor, d *auditpb.AuthzDetail, handlerErr error) *auditpb.AuditEvent {
	evt := rec.buildEvent(ctx, auditpb.AuditEventType_AUDIT_EVENT_TYPE_AUTHZ_DECISION, operation, a)
	evt.Target = &auditpb.AuditTarget{Type: d.GetObjectType(), Id: d.GetObjectId()}
	evt.Detail = &auditpb.AuditEvent_AuthzDetail{AuthzDetail: d}

	allowed := d.GetDecision() == auditpb.AuthzDecision_AUTHZ_DECISION_ALLOWED
	detailErrMsg := ""
	if d.GetDecision() == auditpb.AuthzDecision_AUTHZ_DECISION_ERROR {
		detailErrMsg = d.GetErrorReason()
	}
	evt.Result = resultForDetail(allowed, detailErrMsg, handlerErr)
	return evt
}

// resultForDetail produces an AuditResult given the detail-level success
// flag, the detail-level error message, and the handler-level error.
// Precedence: handler error wins (sets Success=false + ErrorMessage=err.Error());
// otherwise Result reflects detail (Success=detailSuccess, ErrorMessage=detailErrMsg
// only when !detailSuccess).
func resultForDetail(detailSuccess bool, detailErrMsg string, handlerErr error) *auditpb.AuditResult {
	if handlerErr != nil {
		return &auditpb.AuditResult{Success: false, ErrorMessage: handlerErr.Error()}
	}
	r := &auditpb.AuditResult{Success: detailSuccess}
	if !detailSuccess {
		r.ErrorMessage = detailErrMsg
	}
	return r
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
