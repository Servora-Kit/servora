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
