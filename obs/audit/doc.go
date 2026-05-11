// Package audit provides engine-agnostic audit event emission using CloudEvents
// as the envelope format. It defines the Auditor contract and ships MVP backends
// (noop, stdout, kafka, multi) plus a Kratos middleware that intercepts RPC
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
//   - obs/audit/kafka  — delivers events to Kafka via CloudEvents binding (stub)
//   - obs/audit/multi  — fans out to multiple auditors
//
// # Middleware
//
// The Middleware function (audit_middleware.go) intercepts RPC calls, looks up
// CompiledRules by operation, builds CloudEvents events, supplements auth
// metadata, and emits through the configured Auditor. Emission errors are logged
// but never block business logic.
//
// Recommended middleware chain order:
//
//	recovery → tracing → logging → ratelimit → validate → metrics → audit.Middleware → authn → authz → handler
//
// # CloudEvents Extensions
//
// Servora audit events use the following CloudEvents extension attributes
// (defined in extensions.go): authid, authtype, traceparent, tracestate,
// severitytext, recordedtime, partitionkey, errormessage.
package audit
