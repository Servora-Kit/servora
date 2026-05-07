package audit

// Single source of truth for translations between the codegen-friendly Go
// string enums (declared in event.go) and their auditpb counterparts.
//
// All translation helpers live here so future drift between callers (e.g.
// middleware.go, broker_emitter.go, recorder.go) is impossible: add a new
// enum value once in event.go + audit.proto, then extend the matching
// switch below.

import (
	auditpb "github.com/Servora-Kit/servora/api/gen/go/servora/audit/v1"
)

// toProtoEventType maps the codegen-friendly EventType into the proto enum.
func toProtoEventType(t EventType) auditpb.AuditEventType {
	switch t {
	case EventTypeAuthnResult:
		return auditpb.AuditEventType_AUDIT_EVENT_TYPE_AUTHN_RESULT
	case EventTypeAuthzDecision:
		return auditpb.AuditEventType_AUDIT_EVENT_TYPE_AUTHZ_DECISION
	case EventTypeTupleChanged:
		return auditpb.AuditEventType_AUDIT_EVENT_TYPE_TUPLE_CHANGED
	case EventTypeResourceMutation:
		return auditpb.AuditEventType_AUDIT_EVENT_TYPE_RESOURCE_MUTATION
	default:
		return auditpb.AuditEventType_AUDIT_EVENT_TYPE_UNSPECIFIED
	}
}

// toProtoAuthzDecision maps the codegen-friendly AuthzDecision into the proto enum.
func toProtoAuthzDecision(d AuthzDecision) auditpb.AuthzDecision {
	switch d {
	case AuthzDecisionAllowed:
		return auditpb.AuthzDecision_AUTHZ_DECISION_ALLOWED
	case AuthzDecisionDenied:
		return auditpb.AuthzDecision_AUTHZ_DECISION_DENIED
	case AuthzDecisionNoRule:
		return auditpb.AuthzDecision_AUTHZ_DECISION_NO_RULE
	case AuthzDecisionError:
		return auditpb.AuthzDecision_AUTHZ_DECISION_ERROR
	default:
		return auditpb.AuthzDecision_AUTHZ_DECISION_UNSPECIFIED
	}
}

// toProtoTupleMutationType maps the codegen-friendly TupleMutationType into the proto enum.
func toProtoTupleMutationType(t TupleMutationType) auditpb.TupleMutationType {
	switch t {
	case TupleMutationWrite:
		return auditpb.TupleMutationType_TUPLE_MUTATION_TYPE_WRITE
	case TupleMutationDelete:
		return auditpb.TupleMutationType_TUPLE_MUTATION_TYPE_DELETE
	default:
		return auditpb.TupleMutationType_TUPLE_MUTATION_TYPE_UNSPECIFIED
	}
}

// toProtoResourceMutationType maps the codegen-friendly ResourceMutationType into the proto enum.
func toProtoResourceMutationType(t ResourceMutationType) auditpb.ResourceMutationType {
	switch t {
	case ResourceMutationCreate:
		return auditpb.ResourceMutationType_RESOURCE_MUTATION_TYPE_CREATE
	case ResourceMutationUpdate:
		return auditpb.ResourceMutationType_RESOURCE_MUTATION_TYPE_UPDATE
	case ResourceMutationDelete:
		return auditpb.ResourceMutationType_RESOURCE_MUTATION_TYPE_DELETE
	default:
		return auditpb.ResourceMutationType_RESOURCE_MUTATION_TYPE_UNSPECIFIED
	}
}

// eventTypeHeader projects the proto AuditEventType back to the historical
// string header value used by audit pipeline consumers (e.g. broker headers).
func eventTypeHeader(t auditpb.AuditEventType) string {
	switch t {
	case auditpb.AuditEventType_AUDIT_EVENT_TYPE_AUTHN_RESULT:
		return string(EventTypeAuthnResult)
	case auditpb.AuditEventType_AUDIT_EVENT_TYPE_AUTHZ_DECISION:
		return string(EventTypeAuthzDecision)
	case auditpb.AuditEventType_AUDIT_EVENT_TYPE_TUPLE_CHANGED:
		return string(EventTypeTupleChanged)
	case auditpb.AuditEventType_AUDIT_EVENT_TYPE_RESOURCE_MUTATION:
		return string(EventTypeResourceMutation)
	default:
		return ""
	}
}
