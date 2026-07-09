// Package audit provides engine-agnostic audit event emission using CloudEvents
// as the envelope format. It defines the Auditor contract and ships MVP backends
// (noop, stdout, log, kafka, multi) plus a Kratos middleware that intercepts RPC
// calls and emits structured audit events.
//
// # Architecture
//
// The central abstraction is the Auditor interface (auditor.go):
//
//	type Auditor interface {
//	    Emit(ctx context.Context, event cloudevents.Event) error
//	}
//
// Implementations live in sub-packages:
//
//   - obs/audit/noop   — discards all events (testing / disabled mode)
//   - obs/audit/stdout — JSON-encodes events to stdout (local dev)
//   - obs/audit/log    — emits structured slog records (local dev / demos)
//   - obs/audit/kafka  — delivers events to Kafka via franz-go and CloudEvents binding
//   - obs/audit/multi  — fans out to multiple auditors
//
// # Middleware
//
// The Middleware function (audit_middleware.go) intercepts RPC calls, looks up
// generated AuditRules by operation, builds servora.audit.rpc.v1 CloudEvents
// events, and emits through the configured Auditor. Emission errors are logged
// but never block business logic.
//
// Recommended middleware chain order:
//
//	recovery → tracing → logging → ratelimit → validate → metrics → audit.Middleware → authn → authz → handler
//
// # CloudEvents Attributes
//
// NewEvent sets CloudEvents required attributes and uses source="//app-name".
// The generic RPC audit middleware sets subject to the transport operation.
// Extensions are producer-owned: NewEvent adds traceparent/tracestate when a
// sampled OTel span is present, the RPC middleware adds errormessage for handler
// errors, authz emits authid, and backend-specific fields such as partitionkey
// stay private to their backend package.
package audit
