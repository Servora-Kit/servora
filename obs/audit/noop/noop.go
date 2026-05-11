// Package noop provides a no-op Auditor that discards all events silently.
// Use for testing or when audit is disabled.
package noop

import (
	"context"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/Servora-Kit/servora/obs/audit"
)

type auditorImpl struct{}

// NewAuditor returns an Auditor that discards all events without error.
func NewAuditor() audit.Auditor {
	return &auditorImpl{}
}

func (a *auditorImpl) Emit(_ context.Context, _ cloudevents.Event) error {
	return nil
}
