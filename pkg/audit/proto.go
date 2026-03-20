package audit

import (
	"fmt"

	auditv1 "github.com/Servora-Kit/servora/api/gen/go/audit/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func toProtoEvent(event *AuditEvent) (*auditv1.AuditEvent, error) {
	if event == nil {
		return nil, fmt.Errorf("audit: event must not be nil")
	}

	pb := &auditv1.AuditEvent{
		EventId:      event.EventID,
		EventType:    toProtoEventType(event.EventType),
		EventVersion: event.EventVersion,
		OccurredAt:   timestamppb.New(event.OccurredAt),
		Service:      event.Service,
		Operation:    event.Operation,
		Actor: &auditv1.AuditActor{
			Id:          event.Actor.ID,
			Type:        event.Actor.Type,
			DisplayName: event.Actor.DisplayName,
			Email:       event.Actor.Email,
			Subject:     event.Actor.Subject,
			ClientId:    event.Actor.ClientID,
			Realm:       event.Actor.Realm,
		},
		Target: &auditv1.AuditTarget{
			Type: event.Target.Type,
			Id:   event.Target.ID,
			Name: event.Target.Name,
		},
		Result: &auditv1.AuditResult{
			Success:      event.Result.Success,
			ErrorCode:    event.Result.ErrorCode,
			ErrorMessage: event.Result.ErrorMessage,
		},
		TraceId:   event.TraceID,
		RequestId: event.RequestID,
	}

	switch d := event.Detail.(type) {
	case nil:
		return pb, nil
	case AuthnDetail:
		pb.Detail = &auditv1.AuditEvent_AuthnDetail{AuthnDetail: &auditv1.AuthnDetail{
			Method:        d.Method,
			Success:       d.Success,
			FailureReason: d.FailureReason,
		}}
	case *AuthnDetail:
		pb.Detail = &auditv1.AuditEvent_AuthnDetail{AuthnDetail: &auditv1.AuthnDetail{
			Method:        d.Method,
			Success:       d.Success,
			FailureReason: d.FailureReason,
		}}
	case AuthzDetail:
		pb.Detail = &auditv1.AuditEvent_AuthzDetail{AuthzDetail: &auditv1.AuthzDetail{
			Relation:    d.Relation,
			ObjectType:  d.ObjectType,
			ObjectId:    d.ObjectID,
			Decision:    toProtoAuthzDecision(d.Decision),
			CacheHit:    d.CacheHit,
			ErrorReason: d.ErrorReason,
		}}
	case *AuthzDetail:
		pb.Detail = &auditv1.AuditEvent_AuthzDetail{AuthzDetail: &auditv1.AuthzDetail{
			Relation:    d.Relation,
			ObjectType:  d.ObjectType,
			ObjectId:    d.ObjectID,
			Decision:    toProtoAuthzDecision(d.Decision),
			CacheHit:    d.CacheHit,
			ErrorReason: d.ErrorReason,
		}}
	case TupleMutationDetail:
		pb.Detail = &auditv1.AuditEvent_TupleMutationDetail{TupleMutationDetail: toProtoTupleMutationDetail(d)}
	case *TupleMutationDetail:
		pb.Detail = &auditv1.AuditEvent_TupleMutationDetail{TupleMutationDetail: toProtoTupleMutationDetail(*d)}
	case ResourceMutationDetail:
		pb.Detail = &auditv1.AuditEvent_ResourceMutationDetail{ResourceMutationDetail: &auditv1.ResourceMutationDetail{
			MutationType: toProtoResourceMutationType(d.MutationType),
			ResourceType: d.ResourceType,
			ResourceId:   d.ResourceID,
		}}
	case *ResourceMutationDetail:
		pb.Detail = &auditv1.AuditEvent_ResourceMutationDetail{ResourceMutationDetail: &auditv1.ResourceMutationDetail{
			MutationType: toProtoResourceMutationType(d.MutationType),
			ResourceType: d.ResourceType,
			ResourceId:   d.ResourceID,
		}}
	default:
		return nil, fmt.Errorf("audit: unsupported detail type %T", d)
	}

	return pb, nil
}

func toProtoTupleMutationDetail(d TupleMutationDetail) *auditv1.TupleMutationDetail {
	tuples := make([]*auditv1.TupleChange, 0, len(d.Tuples))
	for _, t := range d.Tuples {
		tuples = append(tuples, &auditv1.TupleChange{
			User:     t.User,
			Relation: t.Relation,
			Object:   t.Object,
		})
	}
	return &auditv1.TupleMutationDetail{
		MutationType: toProtoTupleMutationType(d.MutationType),
		Tuples:       tuples,
	}
}

func toProtoEventType(t EventType) auditv1.AuditEventType {
	switch t {
	case EventTypeAuthnResult:
		return auditv1.AuditEventType_AUDIT_EVENT_TYPE_AUTHN_RESULT
	case EventTypeAuthzDecision:
		return auditv1.AuditEventType_AUDIT_EVENT_TYPE_AUTHZ_DECISION
	case EventTypeTupleChanged:
		return auditv1.AuditEventType_AUDIT_EVENT_TYPE_TUPLE_CHANGED
	case EventTypeResourceMutation:
		return auditv1.AuditEventType_AUDIT_EVENT_TYPE_RESOURCE_MUTATION
	default:
		return auditv1.AuditEventType_AUDIT_EVENT_TYPE_UNSPECIFIED
	}
}

func toProtoAuthzDecision(d AuthzDecision) auditv1.AuthzDecision {
	switch d {
	case AuthzDecisionAllowed:
		return auditv1.AuthzDecision_AUTHZ_DECISION_ALLOWED
	case AuthzDecisionDenied:
		return auditv1.AuthzDecision_AUTHZ_DECISION_DENIED
	case AuthzDecisionNoRule:
		return auditv1.AuthzDecision_AUTHZ_DECISION_NO_RULE
	case AuthzDecisionError:
		return auditv1.AuthzDecision_AUTHZ_DECISION_ERROR
	default:
		return auditv1.AuthzDecision_AUTHZ_DECISION_UNSPECIFIED
	}
}

func toProtoTupleMutationType(t TupleMutationType) auditv1.TupleMutationType {
	switch t {
	case TupleMutationWrite:
		return auditv1.TupleMutationType_TUPLE_MUTATION_TYPE_WRITE
	case TupleMutationDelete:
		return auditv1.TupleMutationType_TUPLE_MUTATION_TYPE_DELETE
	default:
		return auditv1.TupleMutationType_TUPLE_MUTATION_TYPE_UNSPECIFIED
	}
}

func toProtoResourceMutationType(t ResourceMutationType) auditv1.ResourceMutationType {
	switch t {
	case ResourceMutationCreate:
		return auditv1.ResourceMutationType_RESOURCE_MUTATION_TYPE_CREATE
	case ResourceMutationUpdate:
		return auditv1.ResourceMutationType_RESOURCE_MUTATION_TYPE_UPDATE
	case ResourceMutationDelete:
		return auditv1.ResourceMutationType_RESOURCE_MUTATION_TYPE_DELETE
	default:
		return auditv1.ResourceMutationType_RESOURCE_MUTATION_TYPE_UNSPECIFIED
	}
}
