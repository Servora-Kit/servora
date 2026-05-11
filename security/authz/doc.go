// Package authz provides a Kratos middleware for engine-agnostic authorization.
//
// # Engine model
//
// The Authorizer interface exposes a single Check method that accepts a
// CheckRequest (Subject, Action, ResourceType, ResourceID, Attributes).
// Any authorization backend — OpenFGA, SpiceDB, Cedar, OPA, or custom —
// can implement this interface.
//
// Batch and list capabilities are available via optional sub-interfaces in
// the batch and lister sub-packages, which engines may implement as needed.
//
// # Subject resolution
//
// The middleware does NOT assume how a subject string is derived from the
// request context. Callers provide a WithSubjectFunc option that extracts
// the subject from ctx. This decouples authz from any specific authn scheme.
//
// # Audit integration
//
// When WithAuditOnDeny is configured with an audit.Auditor, the middleware
// emits CloudEvents events on authorization denial or error. This is purely
// opt-in; without the option, the middleware is silent.
//
// # Future: contextual tuples / attributes
//
// The CheckRequest.Attributes field (map[string]any) is reserved for
// request-level facts that participate in a decision but are not persisted:
// device trust, active session, time-of-day, request region, etc.
// Engines that support ABAC or contextual tuples can read from this field.
package authz
