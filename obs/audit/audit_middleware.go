package audit

import (
	"context"
	"log/slog"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
)

// CompiledRule describes how to audit a single RPC operation.
type CompiledRule struct {
	// Mode is the auditv1.AuditMode int32 value.
	// 0 = UNSPECIFIED (inherit), 1 = DISABLED, 2 = ENABLED.
	Mode int32

	// EventType is the CloudEvents type for the emitted event.
	EventType string

	// Severity is the severity text extension value.
	Severity string

	// BuildEvent constructs the full CloudEvents event for this operation.
	// ctx carries transport and auth metadata; req/resp are handler IO; err is handler error.
	BuildEvent func(ctx context.Context, req, resp any, err error) cloudevents.Event
}

// MiddlewareOption configures the audit middleware.
type MiddlewareOption func(*middlewareConfig)

type middlewareConfig struct {
	subjectFunc  func(context.Context) (string, bool)
	authTypeFunc func(context.Context) (string, bool)
	rulesFuncs   []func() map[string]*CompiledRule
}

// WithSubjectFunc sets a function that extracts the authenticated subject ID from context.
func WithSubjectFunc(fn func(context.Context) (string, bool)) MiddlewareOption {
	return func(c *middlewareConfig) { c.subjectFunc = fn }
}

// WithAuthTypeFunc sets a function that extracts the authentication type (e.g. "jwt", "apikey") from context.
func WithAuthTypeFunc(fn func(context.Context) (string, bool)) MiddlewareOption {
	return func(c *middlewareConfig) { c.authTypeFunc = fn }
}

// WithRulesFuncs registers one or more rule provider functions. Each returns a
// map[operation]*CompiledRule. Multiple providers are merged (later wins on conflict).
func WithRulesFuncs(fns ...func() map[string]*CompiledRule) MiddlewareOption {
	return func(c *middlewareConfig) { c.rulesFuncs = append(c.rulesFuncs, fns...) }
}

// Middleware returns a Kratos middleware that intercepts RPC calls, builds audit
// events according to compiled rules, and emits them through the given Auditor.
// Audit emission errors are logged but never block business logic.
func Middleware(auditor Auditor, opts ...MiddlewareOption) middleware.Middleware {
	cfg := &middlewareConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	// Merge all rule maps once at construction.
	merged := make(map[string]*CompiledRule)
	for _, fn := range cfg.rulesFuncs {
		for op, rule := range fn() {
			merged[op] = rule
		}
	}

	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			// Resolve operation from transport context.
			var operation string
			if tr, ok := transport.FromServerContext(ctx); ok {
				operation = tr.Operation()
			}

			// Look up rule.
			rule, ok := merged[operation]
			if !ok || rule == nil || rule.Mode == 1 { // 1 = DISABLED
				return handler(ctx, req)
			}

			// Call business handler.
			resp, err := handler(ctx, req)

			// Build event.
			event := rule.BuildEvent(ctx, req, resp, err)

			// Supplement auth metadata.
			if cfg.subjectFunc != nil {
				if id, found := cfg.subjectFunc(ctx); found {
					event.SetExtension(ExtAuthID, id)
				}
			}
			if cfg.authTypeFunc != nil {
				if at, found := cfg.authTypeFunc(ctx); found {
					event.SetExtension(ExtAuthType, at)
				}
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
