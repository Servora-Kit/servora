package audit_test

import (
	"context"
	"testing"
	"time"

	auditv1 "github.com/Servora-Kit/servora/api/gen/go/servora/audit/v1"
	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/pkg/actor"
	"github.com/Servora-Kit/servora/pkg/audit"
	"github.com/Servora-Kit/servora/pkg/broker"
	kafkab "github.com/Servora-Kit/servora/pkg/broker/kafka"
	"github.com/Servora-Kit/servora/pkg/logger"
	"google.golang.org/protobuf/proto"
)

func TestE2E_LogEmitter_AuthzDecisionAndTupleChanged(t *testing.T) {
	l := logger.New(nil)
	emitter := audit.NewLogEmitter(l)
	recorder := audit.NewRecorder(emitter, "e2e-test")
	defer func() { _ = recorder.Close() }()

	a := actor.NewUserActor(actor.UserActorParams{
		ID:          "user-e2e-123",
		DisplayName: "E2E User",
		Email:       "e2e@test.com",
	})

	recorder.RecordAuthzDecision(context.Background(), "/test.E2E/Check", a, audit.AuthzDetail{
		Relation:   "viewer",
		ObjectType: "project",
		ObjectID:   "proj-1",
		Decision:   audit.AuthzDecisionAllowed,
		CacheHit:   true,
	})

	recorder.RecordAuthzDecision(context.Background(), "/test.E2E/Check", a, audit.AuthzDetail{
		Relation:   "admin",
		ObjectType: "platform",
		ObjectID:   "default",
		Decision:   audit.AuthzDecisionDenied,
		CacheHit:   false,
	})

	recorder.RecordTupleChange(context.Background(), "openfga.WriteTuples", a, audit.TupleMutationDetail{
		MutationType: audit.TupleMutationWrite,
		Tuples: []audit.TupleChange{
			{User: "user:abc", Relation: "admin", Object: "project:proj-1"},
		},
	})

	recorder.RecordTupleChange(context.Background(), "openfga.DeleteTuples", a, audit.TupleMutationDetail{
		MutationType: audit.TupleMutationDelete,
		Tuples: []audit.TupleChange{
			{User: "user:abc", Relation: "viewer", Object: "organization:org-1"},
		},
	})

	t.Log("LogEmitter e2e: all 4 events emitted without error (check -v output for JSON)")
}

func TestE2E_BrokerEmitter_KafkaRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Kafka integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	kafkaCfg := &conf.Data_Kafka{
		Brokers:       []string{"localhost:29092"},
		ConsumerGroup: "e2e-test-audit-consumer",
	}

	b, err := kafkab.NewBroker(kafkaCfg, nil)
	if err != nil {
		t.Fatalf("failed to create kafka broker: %v", err)
	}
	if err := b.Connect(ctx); err != nil {
		t.Skipf("Kafka not available, skipping: %v", err)
	}
	defer func() { _ = b.Disconnect(ctx) }()

	topic := "e2e-test-audit-events"
	l := logger.New(nil)

	emitter := audit.NewBrokerEmitter(b, topic, l)
	recorder := audit.NewRecorder(emitter, "e2e-test-svc")
	defer func() { _ = recorder.Close() }()

	a := actor.NewUserActor(actor.UserActorParams{
		ID:          "user-kafka-789",
		DisplayName: "Kafka E2E",
	})

	received := make(chan *broker.Message, 1)
	sub, err := b.Subscribe(ctx, topic, func(_ context.Context, event broker.Event) error {
		received <- event.Message()
		return nil
	}, broker.WithQueue("e2e-test-audit-consumer"))
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}
	defer func() { _ = sub.Unsubscribe(true) }()

	time.Sleep(2 * time.Second)

	recorder.RecordAuthzDecision(ctx, "/test.Kafka/Check", a, audit.AuthzDetail{
		Relation:   "admin",
		ObjectType: "platform",
		ObjectID:   "default",
		Decision:   audit.AuthzDecisionDenied,
		CacheHit:   false,
	})

	select {
	case msg := <-received:
		if msg == nil || len(msg.Body) == 0 {
			t.Fatal("received empty message")
		}
		if msg.Headers["event_type"] != "authz.decision" {
			t.Errorf("event_type = %q, want 'authz.decision'", msg.Headers["event_type"])
		}
		if msg.Headers["service"] != "e2e-test-svc" {
			t.Errorf("service = %q, want 'e2e-test-svc'", msg.Headers["service"])
		}
		var pb auditv1.AuditEvent
		if err := proto.Unmarshal(msg.Body, &pb); err != nil {
			t.Fatalf("failed to unmarshal proto: %v", err)
		}
		if pb.GetEventType() != auditv1.AuditEventType_AUDIT_EVENT_TYPE_AUTHZ_DECISION {
			t.Errorf("proto event_type = %v, want AUTHZ_DECISION", pb.GetEventType())
		}
		if pb.GetService() != "e2e-test-svc" {
			t.Errorf("proto service = %q, want 'e2e-test-svc'", pb.GetService())
		}
		if pb.GetActor().GetId() != "user-kafka-789" {
			t.Errorf("proto actor.id = %q, want 'user-kafka-789'", pb.GetActor().GetId())
		}
		authzDetail := pb.GetAuthzDetail()
		if authzDetail == nil {
			t.Fatal("proto authz_detail is nil")
		}
		if authzDetail.GetDecision() != auditv1.AuthzDecision_AUTHZ_DECISION_DENIED {
			t.Errorf("proto decision = %v, want DENIED", authzDetail.GetDecision())
		}
		t.Logf("Kafka e2e: received and decoded audit proto (%d bytes, event_type=%s, service=%s, actor=%s, decision=%s)",
			len(msg.Body), pb.GetEventType(), pb.GetService(), pb.GetActor().GetId(), authzDetail.GetDecision())
	case <-ctx.Done():
		t.Fatal("timed out waiting for Kafka message")
	}
}
