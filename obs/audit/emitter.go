package audit

import (
	"context"

	auditpb "github.com/Servora-Kit/servora/api/gen/go/servora/audit/v1"
)

// Emitter is the interface for sending audit events to a backend.
// Implementations: BrokerEmitter (→ Kafka), LogEmitter (→ logger), NoopEmitter (→ /dev/null).
//
// 事件本体直接使用 auditpb.AuditEvent（proto 为 schema 单源），
// 避免 runtime↔proto 双 schema 与手写 mapper。
type Emitter interface {
	// Emit sends the event to the backend. Errors are non-fatal (audit must not break business flow).
	Emit(ctx context.Context, event *auditpb.AuditEvent) error
	// Close releases any resources held by the emitter.
	Close() error
}
