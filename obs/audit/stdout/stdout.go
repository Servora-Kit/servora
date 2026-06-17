// Package stdout provides an Auditor that JSON-encodes CloudEvents to stdout.
// Useful for local development and debugging.
package stdout

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/Servora-Kit/servora/obs/audit"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// Option configures the stdout auditor.
type Option func(*auditorImpl)

// WithWriter overrides the default os.Stdout destination.
func WithWriter(w io.Writer) Option {
	return func(a *auditorImpl) { a.writer = w }
}

type auditorImpl struct {
	writer io.Writer
}

// NewAuditor returns an Auditor that writes JSON-encoded CloudEvents to stdout.
func NewAuditor(opts ...Option) audit.Auditor {
	a := &auditorImpl{writer: os.Stdout}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

func (a *auditorImpl) Emit(_ context.Context, event cloudevents.Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("stdout auditor: failed to marshal event: %w", err)
	}
	data = append(data, '\n')
	_, err = a.writer.Write(data)
	if err != nil {
		return fmt.Errorf("stdout auditor: failed to write event: %w", err)
	}
	return nil
}
