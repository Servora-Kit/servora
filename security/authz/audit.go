package authz

import (
	"context"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	authzauditpb "github.com/Servora-Kit/servora/api/gen/go/servora/authz/audit/v1"
	"github.com/Servora-Kit/servora/obs/audit"
)

const (
	EventTypeAuthzAllowed = "servora.authz.allowed.v1"
	EventTypeAuthzDenied  = "servora.authz.denied.v1"
	EventTypeAuthzError   = "servora.authz.error.v1"
)

const extAuthID = "authid"

// WithAuditor configures the middleware to emit CloudEvents audit events for
// authorization decisions:
//   - Check returns (true, nil): emit type "servora.authz.allowed.v1".
//   - Check returns (false, nil): emit type "servora.authz.denied.v1".
//   - Check returns (_, err): emit type "servora.authz.error.v1".
//
// Each event carries AuthzDecision protobuf data and the authenticated subject
// in the CloudEvents "authid" extension. No severity extension is emitted.
func WithAuditor(auditor audit.Auditor) Option {
	return func(cfg *serverConfig) { cfg.auditor = auditor }
}

// ceSubject builds the CloudEvents subject from resource type and ID.
func ceSubject(resourceType, resourceID string) string {
	if resourceID == "" {
		return resourceType
	}
	return resourceType + "/" + resourceID
}

// emitAuthzAllowed emits an audit event when authorization succeeds.
// Best-effort: errors are silently ignored.
func emitAuthzAllowed(
	ctx context.Context,
	auditor audit.Auditor,
	subject, action, resourceType, resourceID string,
) {
	if auditor == nil {
		return
	}
	event := audit.NewEvent(ctx,
		audit.WithType(EventTypeAuthzAllowed),
		audit.WithSubject(ceSubject(resourceType, resourceID)),
	)
	setActorExtension(&event, subject)
	data := &authzauditpb.AuthzDecision{
		Decision:     authzauditpb.AuthzDecision_DECISION_ALLOWED,
		Action:       action,
		ResourceType: resourceType,
		ResourceId:   resourceID,
		Reason:       "AUTHZ_ALLOWED",
		Code:         200,
	}
	_ = audit.SetProtoData(&event, data)
	_ = auditor.Emit(ctx, event)
}

// emitAuthzDenied emits an audit event when authorization is denied.
// Best-effort: errors are silently ignored.
func emitAuthzDenied(
	ctx context.Context,
	auditor audit.Auditor,
	subject, action, resourceType, resourceID string,
) {
	if auditor == nil {
		return
	}
	event := audit.NewEvent(ctx,
		audit.WithType(EventTypeAuthzDenied),
		audit.WithSubject(ceSubject(resourceType, resourceID)),
	)
	setActorExtension(&event, subject)
	data := &authzauditpb.AuthzDecision{
		Decision:     authzauditpb.AuthzDecision_DECISION_DENIED,
		Action:       action,
		ResourceType: resourceType,
		ResourceId:   resourceID,
		Reason:       "AUTHZ_DENIED",
		Code:         403,
		Message:      "insufficient permissions",
	}
	_ = audit.SetProtoData(&event, data)
	_ = auditor.Emit(ctx, event)
}

// emitAuthzError emits an audit event when the authorization check itself fails.
// Best-effort: errors are silently ignored.
func emitAuthzError(
	ctx context.Context,
	auditor audit.Auditor,
	subject, action, resourceType, resourceID string,
	checkErr error,
) {
	if auditor == nil {
		return
	}
	event := audit.NewEvent(ctx,
		audit.WithType(EventTypeAuthzError),
		audit.WithSubject(ceSubject(resourceType, resourceID)),
	)
	setActorExtension(&event, subject)
	msg := ""
	if checkErr != nil {
		msg = checkErr.Error()
	}
	data := &authzauditpb.AuthzDecision{
		Decision:     authzauditpb.AuthzDecision_DECISION_ERROR,
		Action:       action,
		ResourceType: resourceType,
		ResourceId:   resourceID,
		Reason:       "AUTHZ_CHECK_FAILED",
		Code:         503,
		Message:      msg,
	}
	_ = audit.SetProtoData(&event, data)
	_ = auditor.Emit(ctx, event)
}

func setActorExtension(event *cloudevents.Event, subject string) {
	if event == nil || subject == "" {
		return
	}
	event.SetExtension(extAuthID, subject)
}
