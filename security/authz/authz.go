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
package authz

import (
	"maps"
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	authzpb "github.com/Servora-Kit/servora/api/gen/go/servora/authz/v1"
	"github.com/Servora-Kit/servora/core/actor"
)

// Authorizer is the interface for checking authorization.
// Implementations are responsible for performing the actual permission check,
// including any caching or backend communication.
type Authorizer interface {
	IsAuthorized(ctx context.Context, subject, relation, objectType, objectID string) (allowed bool, err error)
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

// DecisionDetail describes the result of a single authorization check.
// It is passed to the DecisionLogger callback after every check.
type DecisionDetail struct {
	Operation  string
	Subject    string
	Relation   string
	ObjectType string
	ObjectID   string
	Allowed    bool
	CacheHit   bool
	Err        error
}

// Option configures the Server middleware.
type Option func(*serverConfig)

type serverConfig struct {
	rules          map[string]AuthzRule
	defaultObjID   string
	decisionLogger func(ctx context.Context, detail DecisionDetail)
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
func MergeRules(maps ...map[string]AuthzRule) map[string]AuthzRule {
	total := 0
	for _, m := range maps {
		total += len(m)
	}
	merged := make(map[string]AuthzRule, total)
	for _, m := range maps {
		for k, v := range m {
			merged[k] = v
		}
	}
	return merged
}

// WithDefaultObjectID overrides the fallback object ID used when IDField is empty.
// Defaults to "default".
func WithDefaultObjectID(id string) Option {
	return func(cfg *serverConfig) { cfg.defaultObjID = id }
}

// WithDecisionLogger sets a callback invoked after every authorization check.
// Use this to bridge to audit.Recorder or any other audit sink.
// Replaces the old WithAuditRecorder; keeps pkg/authz free of pkg/audit dependency.
func WithDecisionLogger(fn func(ctx context.Context, detail DecisionDetail)) Option {
	return func(cfg *serverConfig) { cfg.decisionLogger = fn }
}

// Server returns a Kratos middleware that performs authorization checks.
//
// Behavior:
//   - No transport in context → passthrough (non-server calls)
//   - No rule for operation → fail-closed (403 AUTHZ_NO_RULE)
//   - AUTHZ_MODE_NONE → skip (public endpoint)
//   - AUTHZ_MODE_CHECK, no actor or anonymous actor → 403 AUTHZ_DENIED
//   - AUTHZ_MODE_CHECK, nil authorizer → 503 AUTHZ_UNAVAILABLE
//   - AUTHZ_MODE_CHECK, allowed → handler called
//   - AUTHZ_MODE_CHECK, denied → 403 AUTHZ_DENIED
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

			allowed, err := authorizer.IsAuthorized(ctx, principal, relation, objectType, objectID)
			detail := DecisionDetail{
				Operation:  operation,
				Subject:    principal,
				Relation:   relation,
				ObjectType: objectType,
				ObjectID:   objectID,
				Allowed:    allowed,
				Err:        err,
			}

			if cfg.decisionLogger != nil {
				cfg.decisionLogger(ctx, detail)
			}

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

func extractProtoField(req any, fieldName string) (string, error) {
	if fieldName == "" {
		return "", fmt.Errorf("id_field not specified")
	}

	msg, ok := req.(proto.Message)
	if !ok {
		return "", fmt.Errorf("request is not a proto message")
	}

	md := msg.ProtoReflect().Descriptor()
	fd := md.Fields().ByName(protoreflect.Name(fieldName))
	if fd == nil {
		return "", fmt.Errorf("field %q not found in %s", fieldName, md.FullName())
	}

	val := msg.ProtoReflect().Get(fd)
	s := val.String()
	if s == "" {
		return "", fmt.Errorf("field %q is empty", fieldName)
	}
	return s, nil
}
