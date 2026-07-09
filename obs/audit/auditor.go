package audit

import (
	"context"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// Auditor is the engine-agnostic interface for emitting structured audit
// events as CloudEvents. Unlike the legacy Emitter (which takes proto
// AuditEvent), Auditor works directly with the CloudEvents envelope —
// enabling decoupled, transport-neutral audit pipelines.
//
// Implementations may batch, buffer, or fan-out events as needed.
type Auditor interface {
	Emit(ctx context.Context, event cloudevents.Event) error
}

// Closer is an optional interface that Auditor implementations may satisfy
// to release resources on shutdown.
type Closer interface {
	Close() error
}

// Flusher is an optional interface that Auditor implementations may satisfy
// to flush buffered events before graceful shutdown.
type Flusher interface {
	Flush(ctx context.Context) error
}

