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
//	))
//
// The middleware writes a *auditpb.AuthzDetail to ctx via
// audit.WithAuthzResult after every Check (allow / deny / error); emission is
// the responsibility of the transport-tail audit.Collector middleware. The
// authz package therefore has zero coupling to the audit emission pipeline
// (only to the neutral auditpb schema package).
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
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	auditpb "github.com/Servora-Kit/servora/api/gen/go/servora/audit/v1"
	authzpb "github.com/Servora-Kit/servora/api/gen/go/servora/authz/v1"
	"github.com/Servora-Kit/servora/core/actor"
	"github.com/Servora-Kit/servora/obs/audit"
)

// CheckRequest is one item in a BatchCheck call.
type CheckRequest struct {
	Subject    string
	Relation   string
	ObjectType string
	ObjectID   string
}

// CheckResult is the per-item outcome of BatchCheck.
// Order matches the input []CheckRequest index.
type CheckResult struct {
	Allowed bool
	Err     error
}

// Authorizer is the interface for relationship-based authorization decisions.
// All three methods are required: implementations targeting non-ReBAC backends
// (e.g. pure Cedar/Rego) would need a different abstraction entirely, so we
// commit to the ReBAC shape rather than a sub-interface fan-out.
//
// Method names match OpenFGA SDK semantics for direct mapping; SpiceDB
// (LookupResources / BulkCheck) maps cleanly as well.
type Authorizer interface {
	// Check returns whether subject has relation on objectType:objectID.
	Check(ctx context.Context, subject, relation, objectType, objectID string) (allowed bool, err error)

	// BatchCheck runs N checks in one round-trip; output order matches input.
	// Implementations may internally chunk if the backend has per-call limits
	// (OpenFGA caps at 50 per request).
	BatchCheck(ctx context.Context, reqs []CheckRequest) ([]CheckResult, error)

	// ListAllowed returns IDs of objects (of objectType) the subject has the
	// given relation to. The returned strings are bare IDs without "type:" prefix.
	// Useful for "list" endpoints — caller fetches by `WHERE id IN (...)`.
	ListAllowed(ctx context.Context, subject, relation, objectType string) ([]string, error)
}

// AuthzRule describes the authorization requirement for a single RPC operation.
type AuthzRule struct {
	Mode       authzpb.AuthzMode
	Relation   string
	ObjectType string
	// IDField is the proto field name to extract object ID from the request.
	// When empty, "default" is used as the object ID (singleton/platform-level checks).
	IDField string
}

// Option configures the Server middleware.
type Option func(*serverConfig)

type serverConfig struct {
	rules              map[string]AuthzRule
	defaultObjID       string
	checkTimeout       time.Duration
	missingRuleAlertFn func(ctx context.Context, operation string)
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

// WithDefaultObjectID overrides the fallback object ID used when IDField is empty.
// Defaults to "default".
func WithDefaultObjectID(id string) Option {
	return func(cfg *serverConfig) { cfg.defaultObjID = id }
}

// WithCheckTimeout bounds the time spent in Authorizer.Check on each request.
// Zero (default) disables the deadline — the upstream context applies.
//
// This protects business-RPC latency from a slow authorization backend
// (e.g. OpenFGA cross-region calls).
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

// Server returns a Kratos middleware that performs authorization checks.
//
// Behavior:
//   - No transport in context → passthrough (non-server calls); no ctx detail written.
//   - No rule for operation → fail-closed (403 AUTHZ_NO_RULE); no ctx detail written
//     (Authorizer was not invoked, so there is no decision to record).
//   - No rule + WithFailOpenOnMissingRule set → alertFn invoked, handler called;
//     no ctx detail written (same reason).
//   - AUTHZ_MODE_NONE → skip (public endpoint); no ctx detail written.
//   - AUTHZ_MODE_CHECK, no actor or anonymous actor → 403 AUTHZ_DENIED;
//     no ctx detail written (Authorizer was not invoked).
//   - AUTHZ_MODE_CHECK, nil authorizer → 503 AUTHZ_UNAVAILABLE; no ctx detail written.
//   - AUTHZ_MODE_CHECK, authorizer returned (true, nil) → ALLOWED detail in ctx; handler called.
//   - AUTHZ_MODE_CHECK, authorizer returned (false, nil) → DENIED detail in ctx;
//     middleware returns 403 AUTHZ_DENIED.
//   - AUTHZ_MODE_CHECK, authorizer returned (_, err) → ERROR detail in ctx
//     (with ErrorReason=err.Error()); middleware returns 503 AUTHZ_CHECK_FAILED.
//
// In all three Authorizer-invoked outcomes the ctx detail is written BEFORE
// returning, so an OUTER-mounted audit.Collector can observe it post-handler
// even when authz short-circuits.
//
// The OpenFGA principal is constructed as "<actor.Type()>:<actor.ID()>".
func Server(authorizer Authorizer, opts ...Option) middleware.Middleware {
	cfg := &serverConfig{defaultObjID: "default"}
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

			a, ok := actor.FromContext(ctx)
			if !ok || a.Type() == actor.TypeAnonymous {
				return nil, errors.Forbidden("AUTHZ_DENIED", "authentication required")
			}

			if authorizer == nil {
				return nil, errors.ServiceUnavailable("AUTHZ_UNAVAILABLE", "authorization service not available")
			}

			objectType, objectID, err := resolveObject(rule, req, cfg.defaultObjID)
			if err != nil {
				return nil, errors.BadRequest("AUTHZ_BAD_REQUEST",
					fmt.Sprintf("cannot resolve authorization target: %v", err))
			}

			principal := string(a.Type()) + ":" + a.ID()
			relation := rule.Relation

			checkCtx := ctx
			if cfg.checkTimeout > 0 {
				var cancel context.CancelFunc
				checkCtx, cancel = context.WithTimeout(ctx, cfg.checkTimeout)
				defer cancel()
			}

			allowed, err := authorizer.Check(checkCtx, principal, relation, objectType, objectID)

			// Write the decision to ctx BEFORE acting on it: an outer-mounted
			// audit.Collector reads the detail post-handler and emits an
			// AUTHZ_DECISION event regardless of whether this middleware
			// short-circuits on deny / error.
			ctx = audit.WithAuthzResult(ctx, &auditpb.AuthzDetail{
				Relation:    relation,
				ObjectType:  objectType,
				ObjectId:    objectID,
				Decision:    decisionFor(allowed, err),
				ErrorReason: errorReasonFor(err),
			})

			if err != nil {
				return nil, errors.ServiceUnavailable("AUTHZ_CHECK_FAILED",
					fmt.Sprintf("authorization check failed: %v", err))
			}
			if !allowed {
				return nil, errors.Forbidden("AUTHZ_DENIED", "insufficient permissions")
			}

			return handler(ctx, req)
		}
	}
}

// decisionFor maps the (allowed, err) tuple returned by Authorizer.Check into
// the proto-level three-state AuthzDecision enum. Inline here (rather than in
// obs/audit/enums.go) because no runtime-string-enum intermediate exists for
// the (allowed, err) → proto direction; obs/audit's toProtoAuthzDecision goes
// from the runtime AuthzDecision string to proto.
func decisionFor(allowed bool, err error) auditpb.AuthzDecision {
	switch {
	case err != nil:
		return auditpb.AuthzDecision_AUTHZ_DECISION_ERROR
	case allowed:
		return auditpb.AuthzDecision_AUTHZ_DECISION_ALLOWED
	default:
		return auditpb.AuthzDecision_AUTHZ_DECISION_DENIED
	}
}

// errorReasonFor stringifies err for the AuthzDetail.ErrorReason field;
// returns "" when err is nil so allow/deny details have an empty reason.
func errorReasonFor(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// resolveObject determines the FGA object type and ID for the given rule and request.
func resolveObject(rule AuthzRule, req any, defaultObjectID string) (objectType, objectID string, err error) {
	objectType = rule.ObjectType
	if objectType == "" {
		return "", "", fmt.Errorf("object_type not specified in authz rule")
	}

	if rule.IDField == "" {
		return objectType, defaultObjectID, nil
	}

	objectID, err = extractProtoField(req, rule.IDField)
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
		return "", fmt.Errorf("id_field not specified")
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
			return "", fmt.Errorf("field %q is repeated/map; not supported in id_field path", seg)
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
