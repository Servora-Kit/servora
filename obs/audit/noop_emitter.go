package audit

import "context"

// NoopEmitter silently discards all events. Used in tests and when audit is disabled.
type NoopEmitter struct{}

func NewNoopEmitter() *NoopEmitter { return &NoopEmitter{} }

func (n *NoopEmitter) Emit(_ context.Context, _ *AuditEvent) error { return nil }
func (n *NoopEmitter) Close() error                                 { return nil }
