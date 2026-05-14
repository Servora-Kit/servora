package kafka

import (
	"context"
	"fmt"
	"strings"
	"testing"

	brokerv1 "github.com/Servora-Kit/servora/api/gen/go/servora/extra/broker/v1"
	kratoslog "github.com/go-kratos/kratos/v2/log"
)

type captureLogger struct {
	entries []string
}

func (l *captureLogger) Log(_ kratoslog.Level, keyvals ...any) error {
	l.entries = append(l.entries, fmt.Sprint(keyvals...))
	return nil
}

func (l *captureLogger) contains(substr string) bool {
	for _, entry := range l.entries {
		if strings.Contains(entry, substr) {
			return true
		}
	}
	return false
}

func TestNewBrokerOptional_ReturnsNilAndLogsInfoWhenKafkaNotConfigured(t *testing.T) {
	log := &captureLogger{}

	b := NewBrokerOptional(context.Background(), &brokerv1.Broker{}, log)
	if b != nil {
		t.Fatal("expected nil broker when kafka is not configured")
	}
	if !log.contains("Kafka not configured, broker disabled") {
		t.Fatal("expected info log when kafka is not configured")
	}
}

func TestNewBrokerOptional_ReturnsNilAndLogsInfoWhenBrokersEmpty(t *testing.T) {
	log := &captureLogger{}

	cfg := &brokerv1.Broker{Backend: &brokerv1.Broker_Kafka{Kafka: &brokerv1.Kafka{}}}
	b := NewBrokerOptional(context.Background(), cfg, log)
	if b != nil {
		t.Fatal("expected nil broker when kafka brokers are empty")
	}
	if !log.contains("Kafka not configured, broker disabled") {
		t.Fatal("expected info log when kafka brokers are empty")
	}
}

func TestNewBrokerOptional_ReturnsNilInsteadOfPanickingOnInvalidConfig(t *testing.T) {
	log := &captureLogger{}
	cfg := &brokerv1.Broker{
		Backend: &brokerv1.Broker_Kafka{
			Kafka: &brokerv1.Kafka{
				Brokers: []string{"127.0.0.1:9092"},
				Sasl: &brokerv1.Kafka_SASL{
					Mechanism: "INVALID",
					Username:  "u",
					Password:  "p",
				},
			},
		},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("expected NewBrokerOptional not to panic, got %v", r)
		}
	}()

	b := NewBrokerOptional(context.Background(), cfg, log)
	if b != nil {
		t.Fatal("expected nil broker when kafka config is invalid")
	}
	if !log.contains("failed to create Kafka broker") && !log.contains("failed to connect Kafka broker") {
		t.Fatal("expected warning log when kafka config is invalid")
	}
}
