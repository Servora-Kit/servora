// Package multi provides an Auditor that fans out events to multiple backends.
// Emission errors are collected but each backend is called independently —
// one failure does not block the others.
package multi

import (
	"context"
	"errors"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/Servora-Kit/servora/obs/audit"
)

type multiAuditor struct {
	auditors []audit.Auditor
}

// New creates an Auditor that fans out to all provided auditors.
// If no auditors are given, Emit is a no-op.
func New(auditors ...audit.Auditor) audit.Auditor {
	return &multiAuditor{auditors: auditors}
}

func (m *multiAuditor) Emit(ctx context.Context, event cloudevents.Event) error {
	var errs []error
	for _, a := range m.auditors {
		if err := a.Emit(ctx, event); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
