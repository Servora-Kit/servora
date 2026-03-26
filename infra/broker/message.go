package broker

import "context"

// Headers is a string key-value map for message metadata.
type Headers map[string]string

// Message is the unit of data exchanged on a topic.
type Message struct {
	// Key is used for partition routing (e.g. entity ID).
	Key string
	// Headers carries metadata (trace IDs, content-type, schema version, etc.).
	Headers Headers
	// Body is the serialised payload (proto bytes, JSON, etc.).
	Body []byte
	// Partition is the partition this message was received from (consumer-side).
	Partition int32
	// Offset is the offset within the partition (consumer-side).
	Offset int64
}

func (m *Message) GetHeader(key string) string {
	if m.Headers == nil {
		return ""
	}
	return m.Headers[key]
}

// Event wraps an incoming message and provides acknowledge semantics.
// Inspired by kratos-transport/broker Event interface.
type Event interface {
	// Topic returns the topic this event was received from.
	Topic() string
	// Message returns the decoded message.
	Message() *Message
	// RawMessage returns the underlying broker-specific raw message object
	// (e.g. *kgo.Record for Kafka). Useful for advanced use cases.
	RawMessage() any
	// Ack acknowledges successful processing. The broker will not re-deliver.
	Ack() error
	// Nack signals a processing failure. The broker may re-deliver.
	Nack() error
	// Error returns any fetch-level error attached to this event.
	Error() error
}

// Handler is the function signature for message consumers.
type Handler func(ctx context.Context, event Event) error

// MiddlewareFunc wraps a Handler, enabling middleware chains for logging, tracing, retry, etc.
type MiddlewareFunc func(Handler) Handler

// Chain applies multiple middleware functions to a handler (outermost first).
func Chain(h Handler, mws ...MiddlewareFunc) Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// Subscriber represents an active subscription that can be cancelled.
type Subscriber interface {
	// Topic returns the subscribed topic.
	Topic() string
	// Options returns the subscription options that were applied.
	Options() SubscribeOptions
	// Unsubscribe cancels this subscription.
	// Pass removeFromManager=true when called by user code; false when called
	// internally by broker cleanup to avoid double-locking.
	Unsubscribe(removeFromManager bool) error
}
