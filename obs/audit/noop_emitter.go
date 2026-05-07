package audit

import (
	"context"

	auditpb "github.com/Servora-Kit/servora/api/gen/go/servora/audit/v1"
)

// NoopEmitter silently discards all events. Used in tests and when audit is disabled.
type NoopEmitter struct{}

func NewNoopEmitter() *NoopEmitter { return &NoopEmitter{} }

func (n *NoopEmitter) Emit(_ context.Context, _ *auditpb.AuditEvent) error { return nil }
func (n *NoopEmitter) Close() error                                        { return nil }
