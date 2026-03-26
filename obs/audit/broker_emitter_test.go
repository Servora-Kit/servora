package audit

import (
	"context"
	"testing"

	"github.com/Servora-Kit/servora/infra/broker"
	"github.com/Servora-Kit/servora/obs/logging"
)

type stubBrokerEmitterBroker struct {
	published bool
}

func (b *stubBrokerEmitterBroker) Connect(context.Context) error { return nil }

func (b *stubBrokerEmitterBroker) Disconnect(context.Context) error { return nil }

func (b *stubBrokerEmitterBroker) Publish(_ context.Context, _ string, _ *broker.Message, _ ...broker.PublishOption) error {
	b.published = true
	return nil
}

func (b *stubBrokerEmitterBroker) Subscribe(context.Context, string, broker.Handler, ...broker.SubscribeOption) (broker.Subscriber, error) {
	return nil, nil
}

func TestBrokerEmitter_EmitNilEventDoesNotPanicOrPublish(t *testing.T) {
	stubBroker := &stubBrokerEmitterBroker{}
	emitter := NewBrokerEmitter(stubBroker, "", logger.New(nil))

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("expected Emit(nil) not to panic, got %v", r)
		}
	}()

	if err := emitter.Emit(context.Background(), nil); err != nil {
		t.Fatalf("expected nil error for nil event, got %v", err)
	}
	if stubBroker.published {
		t.Fatal("expected nil event not to be published")
	}
}
