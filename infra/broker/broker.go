// Package broker defines Servora's minimal message broker abstraction.
// Inspired by kratos-transport/broker interface design, simplified for Servora's
// event-bus use case (no RPC / Binder semantics).
package broker

import "context"

// Broker is the top-level message broker interface: connect, publish, subscribe.
type Broker interface {
	// Connect establishes a connection to the broker backend.
	Connect(ctx context.Context) error
	// Disconnect closes the connection and releases resources.
	Disconnect(ctx context.Context) error
	// Publish sends a message to the given topic.
	Publish(ctx context.Context, topic string, msg *Message, opts ...PublishOption) error
	// Subscribe registers a handler for messages on the given topic.
	Subscribe(ctx context.Context, topic string, handler Handler, opts ...SubscribeOption) (Subscriber, error)
}
