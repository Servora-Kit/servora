package audit

import (
	"context"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"

	"github.com/Servora-Kit/servora/pkg/actor"
)

// Rule describes how a specific RPC operation should be audited.
type Rule struct {
	// EventType is the audit event type to emit.
	EventType EventType
	// Operation override (defaults to the gRPC operation path).
	Operation string
	// TargetType is the resource type being operated on (e.g. "user").
	TargetType string
	// RecordOnError controls whether to emit events even when the handler fails.
	RecordOnError bool
}

// AuditMiddlewareOption configures the audit middleware.
type AuditMiddlewareOption func(*auditMiddlewareConfig)

type auditMiddlewareConfig struct {
	rules    map[string]Rule // keyed by gRPC operation path
	recorder *Recorder
}

// WithRules sets the per-operation audit rules.
func WithRules(rules map[string]Rule) AuditMiddlewareOption {
	return func(c *auditMiddlewareConfig) { c.rules = rules }
}

// WithRecorder sets the Recorder to use for emitting events.
func WithRecorder(r *Recorder) AuditMiddlewareOption {
	return func(c *auditMiddlewareConfig) { c.recorder = r }
}

// Audit returns a Kratos middleware that records audit events based on configured rules.
// Operations with no matching rule are passed through silently.
//
// This is a skeleton — full implementation follows in phase 2 when audit middleware
// is integrated with real service handlers.
func Audit(opts ...AuditMiddlewareOption) middleware.Middleware {
	cfg := &auditMiddlewareConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	if cfg.recorder == nil {
		cfg.recorder = NewRecorder(NewNoopEmitter(), "")
	}

	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			tr, ok := transport.FromServerContext(ctx)
			if !ok {
				return handler(ctx, req)
			}

			operation := tr.Operation()
			rule, hasRule := cfg.rules[operation]
			if !hasRule {
				return handler(ctx, req)
			}

			// Execute the handler.
			resp, err := handler(ctx, req)

			// Determine whether to emit.
			if err != nil && !rule.RecordOnError {
				return resp, err
			}

			a, _ := actor.FromContext(ctx)
			opName := rule.Operation
			if opName == "" {
				opName = operation
			}

			switch rule.EventType {
			case EventTypeResourceMutation:
				detail := ResourceMutationDetail{
					MutationType: ResourceMutationUpdate,
					ResourceType: rule.TargetType,
				}
				cfg.recorder.RecordResourceMutation(ctx, opName, a, TargetInfo{Type: rule.TargetType}, detail, err)
			}

			return resp, err
		}
	}
}
