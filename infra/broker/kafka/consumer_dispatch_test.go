package kafka

import (
	"context"
	"testing"

	"github.com/Servora-Kit/servora/infra/broker"
	"github.com/twmb/franz-go/pkg/kgo"
)

type ctxProbeKey string

const probeKey ctxProbeKey = "probe"

// TestDispatchPrefersRecordContext is the regression test for the broker
// trace-propagation bug: kotel's OnFetchRecordBuffered writes the upstream
// span context into r.Context, but the previous dispatch implementation
// passed the poll-loop ctx instead, severing distributed trace linkage.
func TestDispatchPrefersRecordContext(t *testing.T) {
	var got context.Context
	s := &kafkaSubscriber{
		handler: func(ctx context.Context, _ broker.Event) error {
			got = ctx
			return nil
		},
		sopts: broker.SubscribeOptions{AutoAck: false},
	}

	rec := &kgo.Record{
		Topic:   "t",
		Context: context.WithValue(context.Background(), probeKey, "from-record"),
	}
	loopCtx := context.WithValue(context.Background(), probeKey, "from-loop")

	s.dispatch(loopCtx, rec)

	if got == nil {
		t.Fatal("handler not invoked")
	}
	if v, _ := got.Value(probeKey).(string); v != "from-record" {
		t.Fatalf("handler received %q, want %q (record ctx must win over loop ctx)", v, "from-record")
	}
}

// TestDispatchFallsBackToLoopContext covers the case where kotel hooks are
// disabled or a record is synthesized without a Context — handler must still
// receive a usable ctx.
func TestDispatchFallsBackToLoopContext(t *testing.T) {
	var got context.Context
	s := &kafkaSubscriber{
		handler: func(ctx context.Context, _ broker.Event) error {
			got = ctx
			return nil
		},
		sopts: broker.SubscribeOptions{AutoAck: false},
	}

	rec := &kgo.Record{Topic: "t"} // Context intentionally nil
	loopCtx := context.WithValue(context.Background(), probeKey, "from-loop")

	s.dispatch(loopCtx, rec)

	if got == nil {
		t.Fatal("handler not invoked")
	}
	if v, _ := got.Value(probeKey).(string); v != "from-loop" {
		t.Fatalf("handler received %q, want %q (nil record ctx must fall back to loop ctx)", v, "from-loop")
	}
}
