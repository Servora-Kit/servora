package kafka

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	brokerv1 "github.com/Servora-Kit/servora/api/gen/go/servora/extra/broker/v1"
)

func testLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})), &buf
}

func TestNewBrokerOptional_ReturnsNilAndLogsInfoWhenKafkaNotConfigured(t *testing.T) {
	log, buf := testLogger()

	b := NewBrokerOptional(context.Background(), &brokerv1.Broker{}, log)
	if b != nil {
		t.Fatal("expected nil broker when kafka is not configured")
	}
	if !strings.Contains(buf.String(), "Kafka not configured") {
		t.Fatalf("expected info log, got: %s", buf.String())
	}
}

func TestNewBrokerOptional_ReturnsNilAndLogsInfoWhenBrokersEmpty(t *testing.T) {
	log, buf := testLogger()

	cfg := &brokerv1.Broker{Backend: &brokerv1.Broker_Kafka{Kafka: &brokerv1.Kafka{}}}
	b := NewBrokerOptional(context.Background(), cfg, log)
	if b != nil {
		t.Fatal("expected nil broker when kafka brokers are empty")
	}
	if !strings.Contains(buf.String(), "Kafka not configured") {
		t.Fatalf("expected info log, got: %s", buf.String())
	}
}

func TestNewBrokerOptional_ReturnsNilInsteadOfPanickingOnInvalidConfig(t *testing.T) {
	log, buf := testLogger()
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
	output := buf.String()
	if !strings.Contains(output, "failed to create Kafka broker") && !strings.Contains(output, "failed to connect Kafka broker") {
		t.Fatalf("expected warning log, got: %s", output)
	}
}
