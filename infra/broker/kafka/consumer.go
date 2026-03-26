package kafka

import (
	"context"
	"fmt"

	"github.com/Servora-Kit/servora/infra/broker"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"
)

// kafkaSubscriber wraps a consumer-group kgo.Client and implements broker.Subscriber.
type kafkaSubscriber struct {
	topic   string
	client  *kgo.Client
	handler broker.Handler
	sopts   broker.SubscribeOptions
	done    chan struct{}
	zap     *zap.Logger
	broker  *kafkaBroker
}

func (s *kafkaSubscriber) Topic() string                  { return s.topic }
func (s *kafkaSubscriber) Options() broker.SubscribeOptions { return s.sopts }

func (s *kafkaSubscriber) Unsubscribe(removeFromManager bool) error {
	select {
	case <-s.done:
	default:
		close(s.done)
	}
	s.client.Close()
	if removeFromManager && s.broker != nil {
		s.broker.removeSub(s)
	}
	return nil
}

// poll is the consumer loop — runs in its own goroutine.
func (s *kafkaSubscriber) poll(ctx context.Context) {
	for {
		select {
		case <-s.done:
			return
		case <-ctx.Done():
			return
		default:
		}

		fetches := s.client.PollFetches(ctx)
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, fe := range errs {
				if s.zap != nil {
					s.zap.Error("kafka consumer fetch error",
						zap.String("topic", fe.Topic),
						zap.Int32("partition", fe.Partition),
						zap.Error(fe.Err),
					)
				}
			}
		}

		fetches.EachRecord(func(r *kgo.Record) {
			event := recordToEvent(r, s.client)
			if err := s.handler(ctx, event); err != nil {
				if s.zap != nil {
					s.zap.Warn("kafka handler error", zap.String("topic", r.Topic), zap.Error(err))
				}
				_ = event.Nack()
				return
			}
			if s.sopts.AutoAck {
				_ = event.Ack()
			}
		})
	}
}

// ── kafkaEvent ────────────────────────────────────────────────────────────────

type kafkaEvent struct {
	record *kgo.Record
	client *kgo.Client
}

func recordToEvent(r *kgo.Record, client *kgo.Client) *kafkaEvent {
	return &kafkaEvent{record: r, client: client}
}

func (e *kafkaEvent) Topic() string { return e.record.Topic }

func (e *kafkaEvent) Message() *broker.Message {
	headers := make(broker.Headers, len(e.record.Headers))
	for _, h := range e.record.Headers {
		headers[h.Key] = string(h.Value)
	}
	key := ""
	if e.record.Key != nil {
		key = string(e.record.Key)
	}
	return &broker.Message{
		Key:     key,
		Headers: headers,
		Body:    e.record.Value,
	}
}

func (e *kafkaEvent) Ack() error {
	e.client.MarkCommitRecords(e.record)
	return nil
}

func (e *kafkaEvent) Nack() error {
	// franz-go does not support individual NACK; return error for consumer-side retry handling.
	return fmt.Errorf("kafka: nack record offset=%d topic=%s partition=%d", e.record.Offset, e.record.Topic, e.record.Partition)
}

// RawMessage returns the underlying *kgo.Record for advanced use cases.
func (e *kafkaEvent) RawMessage() any { return e.record }

// Error returns nil; fetch-level errors are handled in the poll loop, not per-event.
func (e *kafkaEvent) Error() error { return nil }
