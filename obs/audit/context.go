package audit

import (
	"context"

	"go.opentelemetry.io/otel/trace"

	auditpb "github.com/Servora-Kit/servora/api/gen/go/servora/audit/v1"
)

// authnResultKey 与 authzResultKey 是 ctx value 的 sentinel 类型。
// 类型 unexported 保证别的包无法构造同型 key — 这是 Go 社区惯例。
type authnResultKey struct{}
type authzResultKey struct{}

// 写入端固定的 OTel span event 名，与 Collector 的 audit.collected 配对，
// 便于在 trace UI 上看到完整 audit pipeline 时间线。
const (
	spanEventAuthnRecorded = "audit.authn.recorded"
	spanEventAuthzRecorded = "audit.authz.recorded"
)

// WithAuthnResult 将一次认证结果（来自 security/authn middleware）写入 ctx，
// 由 transport 链尾的 audit.Collector 读取并 emit 为 AuditEvent。
// 同时在当前 OTel span 上挂 "audit.authn.recorded" event 便于调试链路。
// ctx 中无 active span 时 AddEvent 是 SDK 内部 noop，不会 panic。
// d == nil 时直接返回原 ctx（不写入、不挂 span event），与"未调用"语义对齐。
func WithAuthnResult(ctx context.Context, d *auditpb.AuthnDetail) context.Context {
	if d == nil {
		return ctx
	}
	trace.SpanFromContext(ctx).AddEvent(spanEventAuthnRecorded)
	return context.WithValue(ctx, authnResultKey{}, d)
}

// AuthnResultFrom 取出由 WithAuthnResult 写入的 detail。未写入返 (nil, false)。
func AuthnResultFrom(ctx context.Context) (*auditpb.AuthnDetail, bool) {
	d, ok := ctx.Value(authnResultKey{}).(*auditpb.AuthnDetail)
	return d, ok
}

// WithAuthzResult 将一次授权决策（来自 security/authz middleware）写入 ctx。
// 同时挂 "audit.authz.recorded" span event。d == nil 时直接返回原 ctx。
func WithAuthzResult(ctx context.Context, d *auditpb.AuthzDetail) context.Context {
	if d == nil {
		return ctx
	}
	trace.SpanFromContext(ctx).AddEvent(spanEventAuthzRecorded)
	return context.WithValue(ctx, authzResultKey{}, d)
}

// AuthzResultFrom 取出由 WithAuthzResult 写入的 detail。
func AuthzResultFrom(ctx context.Context) (*auditpb.AuthzDetail, bool) {
	d, ok := ctx.Value(authzResultKey{}).(*auditpb.AuthzDetail)
	return d, ok
}
