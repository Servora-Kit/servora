package audit

import (
	"context"

	auditpb "github.com/Servora-Kit/servora/api/gen/go/servora/audit/v1"
	"github.com/Servora-Kit/servora/obs/logging"
	"google.golang.org/protobuf/encoding/protojson"
)

// LogEmitter serialises audit events as JSON (proto json) and writes them to the Servora logger.
// Intended for development and debug environments.
type LogEmitter struct {
	log *logger.Helper
}

func NewLogEmitter(l logger.Logger) *LogEmitter {
	return &LogEmitter{log: logger.For(l, "audit/emitter/log")}
}

func (e *LogEmitter) Emit(_ context.Context, event *auditpb.AuditEvent) error {
	if event == nil {
		return nil
	}
	b, err := protojson.Marshal(event)
	if err != nil {
		e.log.Warnf("audit: marshal event: %v", err)
		return nil
	}
	e.log.Infof("audit_event event_id=%s type=%s service=%s operation=%s payload=%s",
		event.GetEventId(), eventTypeHeader(event.GetEventType()), event.GetService(), event.GetOperation(), b)
	return nil
}

func (e *LogEmitter) Close() error { return nil }
