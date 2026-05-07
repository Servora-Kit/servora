package audit

import (
	"context"

	auditpb "github.com/Servora-Kit/servora/api/gen/go/servora/audit/v1"
	"github.com/Servora-Kit/servora/infra/broker"
	logger "github.com/Servora-Kit/servora/obs/logging"
	"google.golang.org/protobuf/proto"
)

const DefaultAuditTopic = "servora.audit.events"

// BrokerEmitter sends audit events to a message broker topic (e.g. Kafka).
// 事件本体已是 *auditpb.AuditEvent，直接 proto.Marshal 即可，无需 runtime↔proto mapper。
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

func (e *BrokerEmitter) Emit(ctx context.Context, event *auditpb.AuditEvent) error {
	if event == nil {
		e.log.Warn("audit: skip nil event")
		return nil
	}

	body, err := proto.Marshal(event)
	if err != nil {
		e.log.Warnf("audit: marshal event %s: %v", event.GetEventId(), err)
		return nil
	}

	msg := &broker.Message{
		Key:  event.GetEventId(),
		Body: body,
		Headers: broker.Headers{
			"content_type":  "application/x-protobuf",
			"event_type":    eventTypeHeader(event.GetEventType()),
			"event_version": event.GetEventVersion(),
			"service":       event.GetService(),
		},
	}

	if err := e.broker.Publish(ctx, e.topic, msg); err != nil {
		e.log.Warnf("audit: publish event %s to topic %s: %v", event.GetEventId(), e.topic, err)
		return nil
	}
	return nil
}

func (e *BrokerEmitter) Close() error { return nil }

// eventTypeHeader 将 proto enum 投影成与历史一致的字符串 header 值。
// 兼容下游既有消费者（如 audit pipeline 按 header 过滤）。
func eventTypeHeader(t auditpb.AuditEventType) string {
	switch t {
	case auditpb.AuditEventType_AUDIT_EVENT_TYPE_AUTHN_RESULT:
		return string(EventTypeAuthnResult)
	case auditpb.AuditEventType_AUDIT_EVENT_TYPE_AUTHZ_DECISION:
		return string(EventTypeAuthzDecision)
	case auditpb.AuditEventType_AUDIT_EVENT_TYPE_TUPLE_CHANGED:
		return string(EventTypeTupleChanged)
	case auditpb.AuditEventType_AUDIT_EVENT_TYPE_RESOURCE_MUTATION:
		return string(EventTypeResourceMutation)
	default:
		return ""
	}
}
