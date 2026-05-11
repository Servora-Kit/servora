package audit

import (
	"context"
	"errors"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// Multi creates a fan-out Auditor that emits events to all provided auditors
// independently. Errors from individual auditors are collected via errors.Join
// but do not block other auditors.
func Multi(auditors ...Auditor) Auditor {
	return &multiAuditor{auditors: auditors}
}

type multiAuditor struct {
	auditors []Auditor
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
