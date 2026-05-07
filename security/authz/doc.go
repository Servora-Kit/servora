// Package authz provides a Kratos middleware for relationship-based authorization.
//
// # Engine model
//
// The Authorizer interface (Check / BatchCheck / ListAllowed) maps directly onto
// OpenFGA SDK and SpiceDB primitives. Both ReBAC backends support all three
// methods natively. The interface is not designed to host non-ReBAC engines
// (Cedar, Rego); those would require a separate abstraction.
//
// # Future: contextual tuples
//
// OpenFGA's "contextual tuples" (and SpiceDB's "caveats") express request-level
// facts that participate in a decision but are not persisted: device trust,
// active session, time-of-day, request region, etc.
//
// When this is needed, the planned API is:
//
//	ctx = authz.WithContextualTuples(ctx, tuples...)        // upstream mw injects
//	authz.ContextualTuplesFromContext(ctx) []Tuple          // engine adapter reads
//
// The Authorizer interface signatures already accept context.Context as the
// first parameter, so no signature change will be required when this is added.
//
// # Audit integration
//
// The Server middleware writes a *auditpb.AuthzDetail to ctx via
// audit.WithAuthzResult after every Authorizer.Check (allow / deny / error).
// An OUTER-mounted audit.Collector middleware (from obs/audit) reads the
// detail post-handler and emits the AUTHZ_DECISION event. This package has
// zero coupling to the audit emission pipeline — only to the neutral auditpb
// schema package.
package authz
