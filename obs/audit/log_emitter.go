package audit

import (
	"context"
	"encoding/json"

	"github.com/Servora-Kit/servora/obs/logging"
)

// LogEmitter serialises audit events as JSON and writes them to the Servora logger.
// Intended for development and debug environments.
type LogEmitter struct {
	log *logger.Helper
}

func NewLogEmitter(l logger.Logger) *LogEmitter {
	return &LogEmitter{log: logger.For(l, "audit/emitter/log")}
}

func (e *LogEmitter) Emit(_ context.Context, event *AuditEvent) error {
	if event == nil {
		return nil
	}
	b, err := json.Marshal(event)
	if err != nil {
		e.log.Warnf("audit: marshal event: %v", err)
		return nil
	}
	e.log.Infof("audit_event event_id=%s type=%s service=%s operation=%s payload=%s",
		event.EventID, event.EventType, event.Service, event.Operation, b)
	return nil
}

func (e *LogEmitter) Close() error { return nil }
