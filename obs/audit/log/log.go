// Package log provides an Auditor that emits CloudEvents through slog.
package log

import (
	"context"
	"log/slog"

	"github.com/Servora-Kit/servora/obs/audit"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

type auditorImpl struct {
	logger *slog.Logger
}

// NewAuditor returns an Auditor that emits structured audit events through l.
func NewAuditor(l *slog.Logger) audit.Auditor {
	return &auditorImpl{logger: l}
}

func (a *auditorImpl) Emit(ctx context.Context, event cloudevents.Event) error {
	// Build the fixed CE required + subject attributes first.
	attrs := []any{
		slog.String("id", event.ID()),
		slog.String("type", event.Type()),
		slog.String("source", event.Source()),
		slog.String("subject", event.Subject()),
		slog.Time("time", event.Time()),
	}

	// Append every extension as a key-value pair without hardcoding names.
	for k, v := range event.Extensions() {
		attrs = append(attrs, slog.Any(k, v))
	}

	a.logger.InfoContext(ctx, "audit_event",
		slog.Group("cloudevent", attrs...),
	)
	return nil
}
