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
//	    pkgauthz.WithRulesFunc(iamv1.AuthzRules),
//	    pkgauthz.WithSubjectFunc(mySubjectExtractor),
//	))
package authz

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	cloudevents "github.com/cloudevents/sdk-go/v2"

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

// AuthzRule describes the authorization requirement for a single RPC operation.
type AuthzRule struct {
	Mode         authzpb.AuthzMode
	Action       string
	ResourceType string
	// ResourceIDField is the proto field name (or dot-path) to extract resource
	// ID from the request. When empty, a default resource ID is used
	// (singleton/platform-level checks).
	ResourceIDField string
}

// Option configures the Server middleware.
type Option func(*serverConfig)

type serverConfig struct {
	rules              map[string]AuthzRule
	defaultResourceID  string
	checkTimeout       time.Duration
	missingRuleAlertFn func(ctx context.Context, operation string)
	subjectFunc        func(context.Context) (string, bool)
	auditOnDeny        audit.Auditor
}

// WithRules sets the operation→rule mapping directly.
func WithRules(rules map[string]AuthzRule) Option {
	return func(cfg *serverConfig) { cfg.rules = rules }
}

// WithRulesFunc sets the operation→rule mapping via a single function (e.g. generated AuthzRules()).
// The function is called once during middleware construction.
// To merge rules from multiple packages, prefer WithRulesFuncs.
func WithRulesFunc(fn func() map[string]AuthzRule) Option {
	return func(cfg *serverConfig) {
		if fn != nil {
			cfg.rules = fn()
		}
	}
}

// WithRulesFuncs merges the rule maps returned by one or more generator functions
// (e.g. userpb.AuthzRules, authnpb.AuthzRules) into a single rule set.
// Later entries take precedence on key conflicts (which should not occur in practice).
// This is the preferred alternative to combining WithRules + MergeRules.
func WithRulesFuncs(fns ...func() map[string]AuthzRule) Option {
	return func(cfg *serverConfig) {
		merged := make(map[string]AuthzRule)
		for _, fn := range fns {
			if fn == nil {
				continue
			}
			maps.Copy(merged, fn())
		}
		cfg.rules = merged
	}
}

// MergeRules merges multiple AuthzRule maps into one new map.
// Later maps take precedence on key conflicts (which should not occur in practice).
// Useful when a server registers services from multiple generated packages.
func MergeRules(ms ...map[string]AuthzRule) map[string]AuthzRule {
	total := 0
	for _, m := range ms {
		total += len(m)
	}
	merged := make(map[string]AuthzRule, total)
	for _, m := range ms {
		maps.Copy(merged, m)
	}
	return merged
}

// WithDefaultResourceID overrides the fallback resource ID used when
// ResourceIDField is empty. Defaults to "default".
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

// WithAuditOnDeny configures the middleware to emit CloudEvents audit events
// when authorization is denied or encounters an error.
//
//   - Check returns (false, nil): emit event type "servora.authz.v1.denied",
//     severity "WARN".
//   - Check returns (_, err): emit event type "servora.authz.v1.denied",
//     severity "ERROR".
//   - Not configured (auditor is nil): silent, no events emitted.
func WithAuditOnDeny(auditor audit.Auditor) Option {
	return func(cfg *serverConfig) { cfg.auditOnDeny = auditor }
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
//   - AUTHZ_MODE_CHECK, authorizer returned (false, nil) → 403 AUTHZ_DENIED;
//     if WithAuditOnDeny is set, emits CloudEvents event with severity WARN.
//   - AUTHZ_MODE_CHECK, authorizer returned (_, err) → 503 AUTHZ_CHECK_FAILED;
//     if WithAuditOnDeny is set, emits CloudEvents event with severity ERROR.
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
			if !found {
				if cfg.missingRuleAlertFn != nil {
					cfg.missingRuleAlertFn(ctx, operation)
					return handler(ctx, req)
				}
				return nil, errors.Forbidden("AUTHZ_NO_RULE",
					fmt.Sprintf("no authorization rule for operation %s", operation))
			}

			if rule.Mode == authzpb.AuthzMode_AUTHZ_MODE_NONE {
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

			checkCtx := ctx
			if cfg.checkTimeout > 0 {
				var cancel context.CancelFunc
				checkCtx, cancel = context.WithTimeout(ctx, cfg.checkTimeout)
				defer cancel()
			}

			allowed, checkErr := authorizer.Check(checkCtx, CheckRequest{
				Subject:      subject,
				Action:       rule.Action,
				ResourceType: resourceType,
				ResourceID:   resourceID,
			})

			if checkErr != nil {
				emitAuditOnDeny(ctx, cfg.auditOnDeny, operation, subject, rule.Action, resourceType, resourceID, checkErr)
				return nil, errors.ServiceUnavailable("AUTHZ_CHECK_FAILED",
					fmt.Sprintf("authorization check failed: %v", checkErr))
			}
			if !allowed {
				emitAuditOnDeny(ctx, cfg.auditOnDeny, operation, subject, rule.Action, resourceType, resourceID, nil)
				return nil, errors.Forbidden("AUTHZ_DENIED", "insufficient permissions")
			}

			return handler(ctx, req)
		}
	}
}

// emitAuditOnDeny emits a CloudEvents audit event when authorization is denied
// or encounters an error. If auditor is nil, this is a no-op.
func emitAuditOnDeny(
	ctx context.Context,
	auditor audit.Auditor,
	operation, subject, action, resourceType, resourceID string,
	checkErr error,
) {
	if auditor == nil {
		return
	}

	e := cloudevents.NewEvent()
	e.SetID(uuid.New().String())
	e.SetType("servora.authz.v1.denied")
	e.SetSource(operation)

	severity := "WARN"
	if checkErr != nil {
		severity = "ERROR"
	}

	payload := map[string]any{
		"subject":       subject,
		"action":        action,
		"resource_type": resourceType,
		"resource_id":   resourceID,
		"severity":      severity,
	}
	if checkErr != nil {
		payload["error"] = checkErr.Error()
	}

	_ = e.SetData(cloudevents.ApplicationJSON, payload)

	// Best-effort: audit emission should not block the authz response.
	_ = auditor.Emit(ctx, e)
}

// resolveResource determines the resource type and ID for the given rule and request.
func resolveResource(rule AuthzRule, req any, defaultResourceID string) (resourceType, resourceID string, err error) {
	resourceType = rule.ResourceType
	if resourceType == "" {
		return "", "", fmt.Errorf("resource_type not specified in authz rule")
	}

	if rule.ResourceIDField == "" {
		return resourceType, defaultResourceID, nil
	}

	resourceID, err = extractProtoField(req, rule.ResourceIDField)
	return
}

// extractProtoField resolves a dot-path against a proto message and returns
// the scalar value at the path's terminus. Constraints:
//   - Each non-leaf segment must be a singular message field (no list/map).
//   - The terminus segment must be a scalar (not a message).
//   - An empty terminus value is treated as an error to preserve the existing
//     "field is required for authorization" contract.
//
// Single-segment paths preserve the prior behavior (top-level scalar lookup).
func extractProtoField(req any, fieldPath string) (string, error) {
	if fieldPath == "" {
		return "", fmt.Errorf("resource_id_field not specified")
	}
	msg, ok := req.(proto.Message)
	if !ok {
		return "", fmt.Errorf("request is not a proto message")
	}

	segments := strings.Split(fieldPath, ".")
	current := msg.ProtoReflect()

	for i, seg := range segments {
		fd := current.Descriptor().Fields().ByName(protoreflect.Name(seg))
		if fd == nil {
			return "", fmt.Errorf("field %q not found in %s",
				seg, current.Descriptor().FullName())
		}
		if fd.IsList() || fd.IsMap() {
			return "", fmt.Errorf("field %q is repeated/map; not supported in resource_id_field path", seg)
		}

		isLast := i == len(segments)-1
		val := current.Get(fd)

		if !isLast {
			// Must be a singular message to traverse further.
			if fd.Kind() != protoreflect.MessageKind {
				return "", fmt.Errorf("path segment %q is scalar but path continues", seg)
			}
			current = val.Message()
			continue
		}

		// Last segment must be a scalar.
		if fd.Kind() == protoreflect.MessageKind {
			return "", fmt.Errorf("path %q terminates on a message field; expected scalar", fieldPath)
		}
		s := val.String()
		if s == "" {
			return "", fmt.Errorf("field %q is empty", fieldPath)
		}
		return s, nil
	}

	return "", fmt.Errorf("unreachable: empty path segments")
}
