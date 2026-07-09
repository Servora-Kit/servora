// Package kafka provides a franz-go backed Auditor and CloudEvents Kafka
// binding helpers.
package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Servora-Kit/servora/obs/audit"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/twmb/franz-go/pkg/kgo"
)

const (
	DefaultTopic                    = "servora.audit.events"
	ContentTypeHeader               = "content-type"
	CloudEventsJSONContentType      = "application/cloudevents+json"
	CloudEventsHeaderPrefix         = "ce_"
	defaultBinaryDataContentType    = "application/octet-stream"
	cloudEventsSpecVersionHeaderKey = "ce_specversion"
	extPartitionKey                 = "partitionkey"
)

type Mode int

const (
	BinaryMode Mode = iota
	StructuredJSONMode
)

// Config holds audit Kafka behavior. Kafka connection construction belongs to
// contrib/kafka; this backend only owns topic, record encoding, and production.
type Config struct {
	Client         *kgo.Client
	Topic          string
	Mode           Mode
	PartitionKeyFn func(cloudevents.Event) string
}

type Option func(*Config)

func WithStructuredMode() Option {
	return func(c *Config) { c.Mode = StructuredJSONMode }
}

func WithPartitionKeyFn(fn func(cloudevents.Event) string) Option {
	return func(c *Config) { c.PartitionKeyFn = fn }
}

type producer interface {
	ProduceSync(context.Context, ...*kgo.Record) kgo.ProduceResults
	Flush(context.Context) error
	Close()
}

type auditorImpl struct {
	cfg      Config
	producer producer
	log      *slog.Logger
}

func NewAuditor(cfg Config, opts ...Option) (audit.Auditor, error) {
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.Topic == "" {
		cfg.Topic = DefaultTopic
	}
	if cfg.PartitionKeyFn == nil {
		cfg.PartitionKeyFn = DefaultPartitionKey
	}
	if cfg.Client == nil {
		return nil, fmt.Errorf("kafka auditor: client is nil")
	}
	return &auditorImpl{
		cfg:      cfg,
		producer: cfg.Client,
		log:      slog.Default().With("scope", "audit/kafka"),
	}, nil
}

func newAuditorWithProducer(cfg Config, p producer, l *slog.Logger, opts ...Option) (audit.Auditor, error) {
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.Topic == "" {
		cfg.Topic = DefaultTopic
	}
	if cfg.PartitionKeyFn == nil {
		cfg.PartitionKeyFn = DefaultPartitionKey
	}
	if p == nil {
		return nil, fmt.Errorf("kafka auditor: producer is nil")
	}
	if l == nil {
		l = slog.Default()
	}
	return &auditorImpl{
		cfg:      cfg,
		producer: p,
		log:      l.With("scope", "audit/kafka"),
	}, nil
}

func (a *auditorImpl) Emit(ctx context.Context, event cloudevents.Event) error {
	record, err := EncodeRecord(a.cfg.Topic, event, a.cfg.Mode, a.cfg.PartitionKeyFn)
	if err != nil {
		return err
	}
	if err := a.producer.ProduceSync(ctx, record).FirstErr(); err != nil {
		return fmt.Errorf("kafka auditor: produce: %w", err)
	}
	return nil
}

func (a *auditorImpl) Flush(ctx context.Context) error {
	return a.producer.Flush(ctx)
}

func (a *auditorImpl) Close() error {
	a.producer.Close()
	return nil
}

func EncodeRecord(topic string, event cloudevents.Event, mode Mode, keyFn func(cloudevents.Event) string) (*kgo.Record, error) {
	if topic == "" {
		return nil, fmt.Errorf("kafka record: topic is required")
	}
	if err := event.Validate(); err != nil {
		return nil, fmt.Errorf("kafka record: invalid CloudEvent: %w", err)
	}
	if keyFn == nil {
		keyFn = DefaultPartitionKey
	}
	switch mode {
	case BinaryMode:
		return encodeBinaryRecord(topic, event, keyFn), nil
	case StructuredJSONMode:
		return encodeStructuredJSONRecord(topic, event, keyFn)
	default:
		return nil, fmt.Errorf("kafka record: unsupported mode %d", mode)
	}
}

func DecodeRecord(record *kgo.Record) (*cloudevents.Event, error) {
	if record == nil {
		return nil, fmt.Errorf("kafka record: nil")
	}
	if strings.HasPrefix(strings.ToLower(header(record.Headers, ContentTypeHeader)), CloudEventsJSONContentType) {
		ev := cloudevents.NewEvent()
		if err := json.Unmarshal(record.Value, &ev); err != nil {
			return nil, fmt.Errorf("structured CloudEvent: %w", err)
		}
		if err := ev.Validate(); err != nil {
			return nil, fmt.Errorf("structured CloudEvent validation: %w", err)
		}
		return &ev, nil
	}

	ev := cloudevents.NewEvent()
	dataContentType := header(record.Headers, ContentTypeHeader)
	for _, h := range record.Headers {
		key := strings.ToLower(h.Key)
		value := string(h.Value)
		switch key {
		case "ce_id":
			ev.SetID(value)
		case "ce_source":
			ev.SetSource(value)
		case cloudEventsSpecVersionHeaderKey:
			ev.SetSpecVersion(value)
		case "ce_type":
			ev.SetType(value)
		case "ce_subject":
			ev.SetSubject(value)
		case "ce_time":
			if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
				ev.SetTime(t)
			}
		case "ce_dataschema":
			ev.SetDataSchema(value)
		case "ce_datacontenttype":
			dataContentType = value
		default:
			if name, ok := strings.CutPrefix(key, CloudEventsHeaderPrefix); ok {
				ev.SetExtension(name, value)
			}
		}
	}
	if ev.SpecVersion() == "" {
		ev.SetSpecVersion(cloudevents.VersionV1)
	}
	if dataContentType == "" {
		dataContentType = defaultBinaryDataContentType
	}
	if len(record.Value) > 0 {
		if err := ev.SetData(dataContentType, record.Value); err != nil {
			return nil, fmt.Errorf("binary CloudEvent data: %w", err)
		}
	}
	if err := ev.Validate(); err != nil {
		return nil, fmt.Errorf("binary CloudEvent validation: %w", err)
	}
	return &ev, nil
}

func encodeBinaryRecord(topic string, event cloudevents.Event, keyFn func(cloudevents.Event) string) *kgo.Record {
	record := &kgo.Record{
		Topic: topic,
		Key:   []byte(keyFn(event)),
		Value: event.Data(),
		Headers: []kgo.RecordHeader{
			{Key: "ce_id", Value: []byte(event.ID())},
			{Key: "ce_source", Value: []byte(event.Source())},
			{Key: cloudEventsSpecVersionHeaderKey, Value: []byte(event.SpecVersion())},
			{Key: "ce_type", Value: []byte(event.Type())},
		},
	}
	if event.Subject() != "" {
		record.Headers = append(record.Headers, kgo.RecordHeader{Key: "ce_subject", Value: []byte(event.Subject())})
	}
	if !event.Time().IsZero() {
		record.Headers = append(record.Headers, kgo.RecordHeader{Key: "ce_time", Value: []byte(event.Time().Format(time.RFC3339Nano))})
	}
	if event.DataSchema() != "" {
		record.Headers = append(record.Headers, kgo.RecordHeader{Key: "ce_dataschema", Value: []byte(event.DataSchema())})
	}
	if event.DataContentType() != "" {
		record.Headers = append(record.Headers,
			kgo.RecordHeader{Key: ContentTypeHeader, Value: []byte(event.DataContentType())},
			kgo.RecordHeader{Key: "ce_datacontenttype", Value: []byte(event.DataContentType())},
		)
	}
	for k, v := range event.Extensions() {
		record.Headers = append(record.Headers, kgo.RecordHeader{
			Key:   CloudEventsHeaderPrefix + strings.ToLower(k),
			Value: []byte(fmt.Sprint(v)),
		})
	}
	return record
}

func encodeStructuredJSONRecord(topic string, event cloudevents.Event, keyFn func(cloudevents.Event) string) (*kgo.Record, error) {
	value, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("structured CloudEvent JSON: %w", err)
	}
	return &kgo.Record{
		Topic: topic,
		Key:   []byte(keyFn(event)),
		Value: value,
		Headers: []kgo.RecordHeader{
			{Key: ContentTypeHeader, Value: []byte(CloudEventsJSONContentType)},
		},
	}, nil
}

func DefaultPartitionKey(event cloudevents.Event) string {
	if v, ok := event.Extensions()[extPartitionKey]; ok {
		if s := fmt.Sprint(v); s != "" {
			return s
		}
	}
	return event.ID()
}

func header(headers []kgo.RecordHeader, key string) string {
	for _, h := range headers {
		if strings.EqualFold(h.Key, key) {
			return string(h.Value)
		}
	}
	return ""
}
