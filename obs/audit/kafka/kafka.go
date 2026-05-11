// Package kafka provides a stub Auditor for Kafka-based audit event delivery.
// The real implementation will wrap cloudevents-sdk-go/protocol/kafka_sarama/v2;
// this stub satisfies the interface and returns an error indicating that Kafka
// support requires the sarama dependency to be configured.
package kafka

import (
	"context"
	"errors"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/Servora-Kit/servora/obs/audit"
)

// ErrNotImplemented is returned by the stub Kafka auditor when Emit is called.
var ErrNotImplemented = errors.New("kafka auditor: not implemented — requires sarama dependency")

// Config holds Kafka connection and behavior settings.
type Config struct {
	// Brokers is the list of Kafka broker addresses.
	Brokers []string

	// Topic is the Kafka topic to produce audit events to.
	Topic string

	// Structured enables CloudEvents structured content mode.
	// Default (false) uses binary content mode.
	Structured bool

	// PartitionKeyFn extracts a partition key from the event.
	// Default: reads event.Extensions()[audit.ExtPartitionKey].
	PartitionKeyFn func(cloudevents.Event) string
}

// Option configures the Kafka auditor.
type Option func(*Config)

// WithStructuredMode switches from binary to structured CloudEvents content mode.
func WithStructuredMode() Option {
	return func(c *Config) { c.Structured = true }
}

// WithPartitionKeyFn sets a custom function to derive the Kafka partition key.
func WithPartitionKeyFn(fn func(cloudevents.Event) string) Option {
	return func(c *Config) { c.PartitionKeyFn = fn }
}

type auditorImpl struct {
	cfg Config
}

// NewAuditor creates a new Kafka-backed Auditor.
// NOTE: This is currently a stub implementation. The real implementation
// requires github.com/cloudevents/sdk-go/protocol/kafka_sarama/v2.
func NewAuditor(cfg Config, opts ...Option) (audit.Auditor, error) {
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.PartitionKeyFn == nil {
		cfg.PartitionKeyFn = defaultPartitionKey
	}
	return &auditorImpl{cfg: cfg}, nil
}

func defaultPartitionKey(e cloudevents.Event) string {
	if v, ok := e.Extensions()[audit.ExtPartitionKey]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return e.ID()
}

func (a *auditorImpl) Emit(_ context.Context, _ cloudevents.Event) error {
	return ErrNotImplemented
}
