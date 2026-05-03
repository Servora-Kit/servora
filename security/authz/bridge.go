package authz

import (
	"context"

	"github.com/Servora-Kit/servora/core/actor"
	"github.com/Servora-Kit/servora/obs/audit"
)

// NewAuthzBridge returns a DecisionLogger that forwards every authorization
// decision to the audit Recorder as an AUTHZ_DECISION event.
//
// Use it as a one-liner in middleware setup:
//
//	authz.Server(authorizer,
//	    authz.WithRulesFunc(rules),
//	    authz.WithDecisionLogger(authz.NewAuthzBridge(recorder)),
//	)
//
// Closes TODO P0-2 (authz → audit auto-bridge).
//
// If recorder is nil, the returned function is a safe no-op.
func NewAuthzBridge(recorder *audit.Recorder) func(context.Context, DecisionDetail) {
	if recorder == nil {
		return func(context.Context, DecisionDetail) {}
	}
	return func(ctx context.Context, d DecisionDetail) {
		a, _ := actor.FromContext(ctx)
		recorder.RecordAuthzDecision(ctx, d.Operation, a, audit.AuthzDetail{
			Relation:    d.Relation,
			ObjectType:  d.ObjectType,
			ObjectID:    d.ObjectID,
			Decision:    decisionFromDetail(d),
			ErrorReason: errorReasonFromDetail(d),
		})
	}
}

func decisionFromDetail(d DecisionDetail) audit.AuthzDecision {
	switch {
	case d.Err != nil:
		return audit.AuthzDecisionError
	case d.Allowed:
		return audit.AuthzDecisionAllowed
	default:
		return audit.AuthzDecisionDenied
	}
}

func errorReasonFromDetail(d DecisionDetail) string {
	if d.Err == nil {
		return ""
	}
	return d.Err.Error()
}
