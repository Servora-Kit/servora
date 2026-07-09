package authn

import (
	"context"

	authnauditpb "github.com/Servora-Kit/servora/api/gen/go/servora/authn/audit/v1"
	"github.com/Servora-Kit/servora/obs/audit"
)

const (
	EventTypeAuthnFailure = "servora.authn.failure.v1"
	EventTypeAuthnSuccess = "servora.authn.success.v1"
)

// WithAuditor installs an Auditor that receives typed CloudEvents for
// authentication outcomes:
//   - failure: type "servora.authn.failure.v1" with AuthnFailure protobuf data.
//   - success: type "servora.authn.success.v1" with AuthnSuccess protobuf data.
//
// NewEvent supplies the CloudEvents required attributes and service source.
// No severity extension is emitted. When no auditor is configured, authn is
// silent.
func WithAuditor(a audit.Auditor) Option {
	return func(c *serverConfig) { c.auditor = a }
}

// emitAuthnFailure constructs a CloudEvents event and emits it via the
// configured Auditor. Errors from Emit are silently ignored.
func emitAuthnFailure(ctx context.Context, auditor audit.Auditor, reason string) {
	if auditor == nil {
		return
	}
	event := audit.NewEvent(ctx, audit.WithType(EventTypeAuthnFailure))
	data := &authnauditpb.AuthnFailure{Reason: reason, Code: 401, Message: reason}
	_ = audit.SetProtoData(&event, data)
	_ = auditor.Emit(ctx, event)
}

// emitAuthnSuccess constructs a CloudEvents success event and emits it via
// the configured Auditor. Errors from Emit are silently ignored.
func emitAuthnSuccess(ctx context.Context, auditor audit.Auditor, scheme string) {
	if auditor == nil {
		return
	}
	event := audit.NewEvent(ctx, audit.WithType(EventTypeAuthnSuccess))
	_ = audit.SetProtoData(&event, &authnauditpb.AuthnSuccess{Scheme: scheme})
	_ = auditor.Emit(ctx, event)
}
