package kafka

import (
	"context"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/pkg/broker"
	"github.com/Servora-Kit/servora/pkg/logger"
	"go.uber.org/zap"
)

// NewBrokerOptional creates a connected Kafka broker from the Data config, or returns
// nil when Kafka is not configured. It follows the optional-initialisation pattern of
// pkg/openfga.NewClientOptional: callers check for nil before use.
func NewBrokerOptional(ctx context.Context, cfg *conf.Data, l logger.Logger) broker.Broker {
	log := logger.For(l, "kafka/broker/pkg")
	if cfg == nil || cfg.Kafka == nil || len(cfg.Kafka.Brokers) == 0 {
		log.Info("Kafka not configured, broker disabled")
		return nil
	}

	zapL := zapFromLogger(l)
	b, err := NewBroker(cfg.Kafka, zapL)
	if err != nil {
		log.Warnf("failed to create Kafka broker: %v", err)
		return nil
	}
	if err := b.Connect(ctx); err != nil {
		log.Warnf("failed to connect Kafka broker: %v", err)
		return nil
	}
	return b
}

// zapFromLogger extracts the underlying *zap.Logger from a logger.Logger.
// Returns nil if the logger is not a *logger.ZapLogger (franz-go will use no-op logging).
func zapFromLogger(l logger.Logger) *zap.Logger {
	if zl, ok := l.(*logger.ZapLogger); ok {
		return zl.Zap()
	}
	return nil
}
