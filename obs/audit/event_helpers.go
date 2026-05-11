package audit

import (
	"context"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

// EventOption configures a CloudEvents event during construction.
type EventOption func(*cloudevents.Event)

// WithType sets the CloudEvents type attribute.
func WithType(t string) EventOption {
	return func(e *cloudevents.Event) { e.SetType(t) }
}

// WithSource sets the CloudEvents source attribute.
func WithSource(s string) EventOption {
	return func(e *cloudevents.Event) { e.SetSource(s) }
}

// WithSeverity sets the severity text extension attribute.
func WithSeverity(s string) EventOption {
	return func(e *cloudevents.Event) { e.SetExtension(ExtSeverityText, s) }
}

// WithSubject sets the CloudEvents subject attribute.
func WithSubject(s string) EventOption {
	return func(e *cloudevents.Event) { e.SetSubject(s) }
}

// NewEvent constructs a CloudEvents event populated with Servora audit defaults.
// It extracts the operation from the transport context (if available) and sets
// it as the source. Options are applied after defaults, allowing full override.
func NewEvent(ctx context.Context, opts ...EventOption) cloudevents.Event {
	e := cloudevents.NewEvent()
	e.SetSpecVersion("1.0")
	e.SetID(uuid.New().String())
	e.SetTime(time.Now())
	e.SetExtension(ExtSeverityText, "INFO")
	e.SetExtension(ExtRecordedTime, time.Now().Format(time.RFC3339Nano))

	// Extract operation from transport context as source.
	if tr, ok := transport.FromServerContext(ctx); ok {
		e.SetSource(tr.Operation())
	}

	for _, opt := range opts {
		opt(&e)
	}

	return e
}

// SetProtoData marshals a protobuf message and sets it as the CloudEvents data
// payload with content type "application/protobuf" and dataschema set to the
// fully qualified protobuf type URL.
func SetProtoData(e *cloudevents.Event, msg proto.Message) error {
	fullName := string(msg.ProtoReflect().Descriptor().FullName())
	e.SetDataSchema("type.googleapis.com/" + fullName)
	e.SetDataContentType("application/protobuf")

	data, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	return e.SetData("application/protobuf", data)
}
