package audit

import (
	"context"

	"github.com/Servora-Kit/servora/pkg/broker"
	"github.com/Servora-Kit/servora/pkg/logger"
	"google.golang.org/protobuf/proto"
)

const DefaultAuditTopic = "servora.audit.events"

// BrokerEmitter sends audit events to a message broker topic (e.g. Kafka).
// Events are proto-marshaled using api/protos/audit/v1/audit.proto.
type BrokerEmitter struct {
	broker broker.Broker
	topic  string
	log    *logger.Helper
}

func NewBrokerEmitter(b broker.Broker, topic string, l logger.Logger) *BrokerEmitter {
	if topic == "" {
		topic = DefaultAuditTopic
	}
	return &BrokerEmitter{
		broker: b,
		topic:  topic,
		log:    logger.For(l, "audit/emitter/broker"),
	}
}

func (e *BrokerEmitter) Emit(ctx context.Context, event *AuditEvent) error {
	if event == nil {
		e.log.Warn("audit: skip nil event")
		return nil
	}

	pb, err := toProtoEvent(event)
	if err != nil {
		e.log.Warnf("audit: convert event %s to proto: %v", event.EventID, err)
		return nil
	}
	body, err := proto.Marshal(pb)
	if err != nil {
		e.log.Warnf("audit: marshal event %s: %v", event.EventID, err)
		return nil
	}

	msg := &broker.Message{
		Key:  event.EventID,
		Body: body,
		Headers: broker.Headers{
			"content_type":  "application/x-protobuf",
			"event_type":    string(event.EventType),
			"event_version": event.EventVersion,
			"service":       event.Service,
		},
	}

	if err := e.broker.Publish(ctx, e.topic, msg); err != nil {
		e.log.Warnf("audit: publish event %s to topic %s: %v", event.EventID, e.topic, err)
		return nil
	}
	return nil
}

func (e *BrokerEmitter) Close() error { return nil }
