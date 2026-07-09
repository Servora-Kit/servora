// Package authz provides a generic Kratos middleware for authorization.
// It is engine-agnostic: any Authorizer implementation can be injected.
//
// Example usage:
//
//	import (
//	    pkgauthz "github.com/Servora-Kit/servora/security/authz"
//	    fgaengine "github.com/Servora-Kit/servora/security/authz/openfga"
//	)
//
//	mw = append(mw, pkgauthz.Server(
//	    fgaengine.NewAuthorizer(fgaClient),
//	    pkgauthz.WithRulesFuncs(iamv1.AuthzRules),
//	    pkgauthz.WithSubjectFunc(mySubjectExtractor),
//	))
package authz

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/transport"

	authzpb "github.com/Servora-Kit/servora/api/gen/go/servora/authz/v1"
	"github.com/Servora-Kit/servora/obs/audit"
)

// Authorizer is the single-method interface for authorization decisions.
// Implementations may target any backend (OpenFGA, SpiceDB, Cedar, OPA, etc.).
type Authorizer interface {
	// Check returns whether the request described by req is authorized.
	Check(ctx context.Context, req CheckRequest) (allowed bool, err error)
}

// CheckRequest describes a single authorization check.
type CheckRequest struct {
	Subject      string
	Action       string
	ResourceType string
	ResourceID   string
	Attributes   map[string]any
}

// Option configures the Server middleware.
type Option func(*serverConfig)

type serverConfig struct {
	rules              map[string]*authzpb.AuthzRule
	defaultResourceID  string
	checkTimeout       time.Duration
	missingRuleAlertFn func(ctx context.Context, operation string)
	subjectFunc        func(context.Context) (string, bool)
	auditor            audit.Auditor
}

// WithRulesFuncs merges the rule maps returned by one or more generator functions
// (e.g. userpb.AuthzRules, iampb.AuthzRules) into a single rule set.
// Later entries take precedence on key conflicts.
func WithRulesFuncs(fns ...func() map[string]*authzpb.AuthzRule) Option {
	return func(cfg *serverConfig) {
		for _, fn := range fns {
			if fn == nil {
				continue
			}
			rules := fn()
			if len(rules) == 0 {
				continue
			}
			if cfg.rules == nil {
				cfg.rules = make(map[string]*authzpb.AuthzRule, len(rules))
			}
			for op, rule := range rules {
				if rule == nil {
					continue
				}
				cfg.rules[op] = rule
			}
		}
	}
}

// WithDefaultResourceID overrides the fallback resource ID used when
// resource_id_field is empty. Defaults to "default".
func WithDefaultResourceID(id string) Option {
	return func(cfg *serverConfig) { cfg.defaultResourceID = id }
}

// WithCheckTimeout bounds the time spent in Authorizer.Check on each request.
// Zero (default) disables the deadline — the upstream context applies.
//
// This protects business-RPC latency from a slow authorization backend.
func WithCheckTimeout(d time.Duration) Option {
	return func(cfg *serverConfig) { cfg.checkTimeout = d }
}

// WithFailOpenOnMissingRule changes the missing-rule policy from fail-closed
// (default — return AUTHZ_NO_RULE 403) to fail-open: the handler is called,
// and the alertFn callback is invoked so the gap is visible (oncall page,
// Slack, log warning, etc.).
//
// Use during development or staged rollouts. NEVER use in production for
// security-sensitive services.
func WithFailOpenOnMissingRule(alertFn func(ctx context.Context, operation string)) Option {
	return func(cfg *serverConfig) { cfg.missingRuleAlertFn = alertFn }
}

// WithSubjectFunc sets the function used to extract the subject string from
// the request context. The function should return the subject identifier and
// a boolean indicating whether the subject was found. When not set or when
// the function returns false, the middleware returns 403 AUTHZ_DENIED.
func WithSubjectFunc(fn func(context.Context) (string, bool)) Option {
	return func(cfg *serverConfig) { cfg.subjectFunc = fn }
}

// Server returns a Kratos middleware that performs authorization checks.
//
// Behavior:
//   - No transport in context → passthrough (non-server calls).
//   - No rule for operation → fail-closed (403 AUTHZ_NO_RULE).
//   - No rule + WithFailOpenOnMissingRule set → alertFn invoked, handler called.
//   - AUTHZ_MODE_NONE → skip (public endpoint).
//   - AUTHZ_MODE_CHECK, no subject → 403 AUTHZ_DENIED.
//   - AUTHZ_MODE_CHECK, nil authorizer → 503 AUTHZ_UNAVAILABLE.
//   - AUTHZ_MODE_CHECK, authorizer returned (true, nil) → handler called.
//     if WithAuditor is set, emits servora.authz.allowed.v1.
//   - AUTHZ_MODE_CHECK, authorizer returned (false, nil) → 403 AUTHZ_DENIED;
//     if WithAuditor is set, emits servora.authz.denied.v1.
//   - AUTHZ_MODE_CHECK, authorizer returned (_, err) → 503 AUTHZ_CHECK_FAILED;
//     if WithAuditor is set, emits servora.authz.error.v1.
func Server(authorizer Authorizer, opts ...Option) middleware.Middleware {
	cfg := &serverConfig{defaultResourceID: "default"}
	for _, o := range opts {
		o(cfg)
	}

	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			tr, ok := transport.FromServerContext(ctx)
			if !ok {
				return handler(ctx, req)
			}

			operation := tr.Operation()
			rule, found := cfg.rules[operation]
			if !found || rule == nil {
				if cfg.missingRuleAlertFn != nil {
					cfg.missingRuleAlertFn(ctx, operation)
					return handler(ctx, req)
				}
				return nil, errors.Forbidden("AUTHZ_NO_RULE",
					fmt.Sprintf("no authorization rule for operation %s", operation))
			}

			if rule.GetMode() == authzpb.AuthzMode_AUTHZ_MODE_NONE {
				return handler(ctx, req)
			}

			subject, ok := "", false
			if cfg.subjectFunc != nil {
				subject, ok = cfg.subjectFunc(ctx)
			}
			if !ok || subject == "" {
				return nil, errors.Forbidden("AUTHZ_DENIED", "authentication required")
			}

			if authorizer == nil {
				return nil, errors.ServiceUnavailable("AUTHZ_UNAVAILABLE", "authorization service not available")
			}

			resourceType, resourceID, err := resolveResource(rule, req, cfg.defaultResourceID)
			if err != nil {
				return nil, errors.BadRequest("AUTHZ_BAD_REQUEST",
					fmt.Sprintf("cannot resolve authorization target: %v", err))
			}
			action := rule.GetAction()

			checkCtx := ctx
			if cfg.checkTimeout > 0 {
				var cancel context.CancelFunc
				checkCtx, cancel = context.WithTimeout(ctx, cfg.checkTimeout)
				defer cancel()
			}

			allowed, checkErr := authorizer.Check(checkCtx, CheckRequest{
				Subject:      subject,
				Action:       action,
				ResourceType: resourceType,
				ResourceID:   resourceID,
			})

			if checkErr != nil {
				emitAuthzError(ctx, cfg.auditor, subject, action, resourceType, resourceID, checkErr)
				return nil, errors.ServiceUnavailable("AUTHZ_CHECK_FAILED",
					fmt.Sprintf("authorization check failed: %v", checkErr))
			}
			if !allowed {
				emitAuthzDenied(ctx, cfg.auditor, subject, action, resourceType, resourceID)
				return nil, errors.Forbidden("AUTHZ_DENIED", "insufficient permissions")
			}

			emitAuthzAllowed(ctx, cfg.auditor, subject, action, resourceType, resourceID)
			return handler(ctx, req)
		}
	}
}
