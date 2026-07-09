package audit

import (
	"context"
	"fmt"
	"time"

	kratos "github.com/go-kratos/kratos/v3"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"
)

// OTel W3C trace context CE extension attribute names (private — not part of public API).
const (
	extTraceParent = "traceparent"
	extTraceState  = "tracestate"
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

// WithSubject sets the CloudEvents subject attribute.
func WithSubject(s string) EventOption {
	return func(e *cloudevents.Event) { e.SetSubject(s) }
}

// NewEvent constructs a CloudEvents event populated with Servora audit defaults.
// It extracts the app name from context (if available) and sets it as the source
// in the format "//name", falling back to "//unknown". Options are applied after
// defaults, allowing full override.
func NewEvent(ctx context.Context, opts ...EventOption) cloudevents.Event {
	e := cloudevents.NewEvent()
	e.SetSpecVersion("1.0")
	e.SetID(uuid.New().String())
	e.SetTime(time.Now())

	// Extract app name from context as source.
	if info, ok := kratos.FromContext(ctx); ok {
		e.SetSource("//" + info.Name())
	} else {
		e.SetSource("//unknown")
	}

	// Inject OTel trace context if span is valid and sampled.
	if span := trace.SpanFromContext(ctx); span != nil {
		sc := span.SpanContext()
		if sc.IsValid() && sc.IsSampled() {
			traceparent := fmt.Sprintf("00-%s-%s-%02x",
				sc.TraceID().String(),
				sc.SpanID().String(),
				uint8(sc.TraceFlags()),
			)
			e.SetExtension(extTraceParent, traceparent)
			if ts := sc.TraceState().String(); ts != "" {
				e.SetExtension(extTraceState, ts)
			}
		}
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
