package audit

import (
	"context"
	"log/slog"

	auditv1 "github.com/Servora-Kit/servora/api/gen/go/servora/audit/v1"
	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/transport"
)

// Option configures the audit middleware.
type Option func(*serverConfig)

// extErrorMessage is the CE extension key for handler error summaries (private).
const extErrorMessage = "errormessage"

type serverConfig struct {
	rulesFuncs []func() map[string]*auditv1.AuditRule
}

// WithRulesFuncs registers one or more rule provider functions. Each returns a
// map[operation]*auditv1.AuditRule. Multiple providers are merged (later wins on conflict).
func WithRulesFuncs(fns ...func() map[string]*auditv1.AuditRule) Option {
	return func(c *serverConfig) { c.rulesFuncs = append(c.rulesFuncs, fns...) }
}

// Middleware returns a Kratos middleware that intercepts RPC calls, constructs
// a generic RPC audit event when the operation's rule is ENABLED, and emits it
// through the given Auditor. Audit emission errors are logged but never block
// business logic.
func Middleware(auditor Auditor, opts ...Option) middleware.Middleware {
	cfg := &serverConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	// Merge all rule maps once at construction.
	merged := make(map[string]*auditv1.AuditRule)
	for _, fn := range cfg.rulesFuncs {
		for op, rule := range fn() {
			merged[op] = rule
		}
	}

	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			// Resolve operation from transport context.
			var operation string
			if tr, ok := transport.FromServerContext(ctx); ok {
				operation = tr.Operation()
			}

			// Look up rule; skip if not found or disabled.
			rule, ok := merged[operation]
			if !ok || rule == nil || rule.GetMode() != auditv1.AuditMode_AUDIT_MODE_ENABLED {
				return handler(ctx, req)
			}

			// Call business handler.
			resp, err := handler(ctx, req)

			// Construct a generic RPC audit event.
			event := NewEvent(ctx,
				WithType("servora.audit.rpc.v1"),
				WithSubject(operation),
			)

			// Attach handler error summary if present.
			if err != nil {
				event.SetExtension(extErrorMessage, err.Error())
			}

			// Emit — log error but never block business logic.
			if emitErr := auditor.Emit(ctx, event); emitErr != nil {
				slog.ErrorContext(ctx, "audit emit failed",
					slog.String("operation", operation),
					slog.String("error", emitErr.Error()),
				)
			}

			return resp, err
		}
	}
}

// Collector is the ChainBuilder-facing alias for the audit middleware.
func Collector(auditor Auditor, opts ...Option) middleware.Middleware {
	return Middleware(auditor, opts...)
}
