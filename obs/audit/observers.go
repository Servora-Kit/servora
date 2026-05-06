// Package audit observers — bridges between security/{authn,authz}
// observer callbacks and the audit Recorder.
//
// This file is the ONLY place in obs/audit that imports security/authn
// or security/authz. The dependency direction is intentional:
//
//	obs/audit  →  security/authn
//	obs/audit  →  security/authz
//
// security packages remain free of any dependency on obs/audit; audit
// observes them, not the other way around. See plan.md "Decisions" §2.

package audit

import (
	"context"

	"github.com/go-kratos/kratos/v2/transport"

	"github.com/Servora-Kit/servora/core/actor"
	"github.com/Servora-Kit/servora/security/authn"
	"github.com/Servora-Kit/servora/security/authz"
)

// AuthnObserver returns a callback for security/authn.Server's
// WithObserver option that forwards every authentication outcome to
// this Recorder via RecordAuthnResult.
//
// The returned callback is nil-safe: when invoked on a nil *Recorder,
// it is a no-op, allowing business code to wire an observer
// unconditionally without first nil-checking the recorder.
//
// Field mapping (security/authn.AuthnDetail → audit.AuthnDetail):
//   - Method      → Method (verbatim)
//   - Allowed     → Success
//   - Err.Error() → FailureReason (empty string when Err == nil)
//
// Subject (actor.Actor) is passed as the audit Actor; Operation is
// resolved from the request transport (empty when no transport).
func (r *Recorder) AuthnObserver() func(context.Context, authn.AuthnDetail) {
	if r == nil {
		return func(context.Context, authn.AuthnDetail) {}
	}
	return func(ctx context.Context, d authn.AuthnDetail) {
		r.RecordAuthnResult(ctx, operationFromContext(ctx), d.Subject, AuthnDetail{
			Method:        d.Method,
			Success:       d.Allowed,
			FailureReason: errReason(d.Err),
		})
	}
}

// AuthzObserver returns a callback for security/authz.Server's
// WithObserver option that forwards every Check decision to this
// Recorder via RecordAuthzDecision.
//
// The returned callback is nil-safe: when invoked on a nil *Recorder,
// it is a no-op.
//
// Decision mapping (authz.DecisionDetail → audit.AuthzDetail.Decision):
//   - d.Err != nil   → AuthzDecisionError
//   - d.Allowed      → AuthzDecisionAllowed
//   - !d.Allowed     → AuthzDecisionDenied
//
// The audit Actor is extracted from ctx via actor.FromContext (mirroring
// the previous bridge.go behavior); fall back to anonymous when not set.
func (r *Recorder) AuthzObserver() func(context.Context, authz.DecisionDetail) {
	if r == nil {
		return func(context.Context, authz.DecisionDetail) {}
	}
	return func(ctx context.Context, d authz.DecisionDetail) {
		a, ok := actor.FromContext(ctx)
		if !ok {
			a = actor.NewAnonymousActor()
		}
		r.RecordAuthzDecision(ctx, d.Operation, a, AuthzDetail{
			Relation:    d.Relation,
			ObjectType:  d.ObjectType,
			ObjectID:    d.ObjectID,
			Decision:    decisionFromAuthz(d),
			ErrorReason: errReason(d.Err),
		})
	}
}

// decisionFromAuthz maps the authz callback tri-state into AuthzDecision.
// Order matters: error wins over allowed/denied because an engine error
// is not equivalent to a deny — downstream consumers (alerting, SLO)
// must distinguish the two.
func decisionFromAuthz(d authz.DecisionDetail) AuthzDecision {
	switch {
	case d.Err != nil:
		return AuthzDecisionError
	case d.Allowed:
		return AuthzDecisionAllowed
	default:
		return AuthzDecisionDenied
	}
}

func errReason(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func operationFromContext(ctx context.Context) string {
	if tr, ok := transport.FromServerContext(ctx); ok {
		return tr.Operation()
	}
	return ""
}
