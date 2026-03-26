package broker

import "maps"

// PublishOption configures a single Publish call.
type PublishOption func(*PublishOptions)

// PublishOptions holds all per-publish settings.
type PublishOptions struct {
	// Headers merged into the message headers (override message.Headers on conflict).
	Headers Headers
}

// WithPublishHeaders adds extra headers to a published message.
func WithPublishHeaders(h Headers) PublishOption {
	return func(o *PublishOptions) {
		if o.Headers == nil {
			o.Headers = make(Headers)
		}
		maps.Copy(o.Headers, h)
	}
}

// SubscribeOption configures a Subscribe call.
type SubscribeOption func(*SubscribeOptions)

// SubscribeOptions holds all subscription settings.
type SubscribeOptions struct {
	// AutoAck automatically calls Ack after the handler returns without error.
	// Defaults to true.
	AutoAck bool
	// Queue enables competing-consumer (queue group / consumer group) semantics.
	// Multiple subscribers with the same Queue value share the load.
	Queue string
	// Middlewares wraps the handler with a chain of MiddlewareFuncs (outermost first).
	Middlewares []MiddlewareFunc
}

// NewSubscribeOptions creates SubscribeOptions with defaults applied.
func NewSubscribeOptions(opts ...SubscribeOption) SubscribeOptions {
	o := SubscribeOptions{AutoAck: true}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// DisableAutoAck disables automatic Ack on handler success.
// The handler is then responsible for calling Event.Ack() or Event.Nack().
func DisableAutoAck() SubscribeOption {
	return func(o *SubscribeOptions) { o.AutoAck = false }
}

// WithAutoAck explicitly sets auto-ack mode (use DisableAutoAck() for the common disable case).
func WithAutoAck(v bool) SubscribeOption {
	return func(o *SubscribeOptions) { o.AutoAck = v }
}

// WithQueue sets the consumer group / queue group name for competing-consumer delivery.
func WithQueue(name string) SubscribeOption {
	return func(o *SubscribeOptions) { o.Queue = name }
}

// WithMiddlewares adds handler middleware functions to the subscription.
func WithMiddlewares(mws ...MiddlewareFunc) SubscribeOption {
	return func(o *SubscribeOptions) { o.Middlewares = append(o.Middlewares, mws...) }
}
