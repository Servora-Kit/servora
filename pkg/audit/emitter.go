package audit

import "context"

// Emitter is the interface for sending audit events to a backend.
// Implementations: BrokerEmitter (→ Kafka), LogEmitter (→ logger), NoopEmitter (→ /dev/null).
type Emitter interface {
	// Emit sends the event to the backend. Errors are non-fatal (audit must not break business flow).
	Emit(ctx context.Context, event *AuditEvent) error
	// Close releases any resources held by the emitter.
	Close() error
}
