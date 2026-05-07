// Package audit provides Servora's audit event runtime: emitter interface,
// recorder, and Kratos middleware skeleton. Schema is sourced from
// api/protos/servora/audit/v1/audit.proto (auditpb).
//
// 本文件仅保留 codegen-friendly 的 Go 字符串枚举，
// 它们用于 audit middleware 的 Rule struct（基于 proto 注解生成）。
// 事件本体与各 detail 均使用 auditpb.* 而非 runtime struct（schema 单源 = proto）。
package audit

// EventType categorizes audit events. Used by audit middleware Rule.
type EventType string

const (
	EventTypeAuthnResult      EventType = "authn.result"
	EventTypeAuthzDecision    EventType = "authz.decision"
	EventTypeTupleChanged     EventType = "tuple.changed"
	EventTypeResourceMutation EventType = "resource.mutation"
)

// AuthzDecision describes the outcome of an authorization check.
//
// 该 Go 字符串别名仅在 obs/audit 包内部用作可读语义，
// 不进入对外 schema；对外契约一律走 auditpb.AuthzDecision (proto enum)。
type AuthzDecision string

const (
	AuthzDecisionAllowed AuthzDecision = "allowed"
	AuthzDecisionDenied  AuthzDecision = "denied"
	AuthzDecisionNoRule  AuthzDecision = "no_rule"
	AuthzDecisionError   AuthzDecision = "error"
)

// TupleMutationType describes the type of OpenFGA tuple change.
// Used by Rule struct in audit middleware (codegen-friendly).
type TupleMutationType string

const (
	TupleMutationWrite  TupleMutationType = "write"
	TupleMutationDelete TupleMutationType = "delete"
)

// ResourceMutationType describes the type of resource mutation.
// Used by Rule struct in audit middleware (codegen-friendly).
type ResourceMutationType string

const (
	ResourceMutationCreate ResourceMutationType = "create"
	ResourceMutationUpdate ResourceMutationType = "update"
	ResourceMutationDelete ResourceMutationType = "delete"
)
