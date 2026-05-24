package kafka

import (
	"context"
	"log/slog"

	brokerv1 "github.com/Servora-Kit/servora/api/gen/go/servora/extra/broker/v1"
	"github.com/Servora-Kit/servora/infra/broker"
)

// NewBrokerOptional creates a connected Kafka broker from the Broker section,
// or returns nil when Kafka is not configured. Callers check for nil before use.
func NewBrokerOptional(ctx context.Context, cfg *brokerv1.Broker, l *slog.Logger) broker.Broker {
	log := l.With("scope", "kafka/broker/infra")
	if cfg == nil || cfg.GetKafka() == nil || len(cfg.GetKafka().GetBrokers()) == 0 {
		log.Info("Kafka not configured, broker disabled")
		return nil
	}

	b, err := NewBroker(cfg.GetKafka(), nil)
	if err != nil {
		log.Warn("failed to create Kafka broker", "err", err)
		return nil
	}
	if err := b.Connect(ctx); err != nil {
		log.Warn("failed to connect Kafka broker", "err", err)
		return nil
	}
	return b
}
