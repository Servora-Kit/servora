package logger

import (
	"context"
	"log/slog"
	"sync"
	"testing"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

type mockExporter struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (e *mockExporter) Export(_ context.Context, records []sdklog.Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.records = append(e.records, records...)
	return nil
}

func (e *mockExporter) Shutdown(context.Context) error { return nil }
func (e *mockExporter) ForceFlush(context.Context) error { return nil }

func TestOtelHandler_TraceCorrelation(t *testing.T) {
	exp := &mockExporter{}
	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewSimpleProcessor(exp)),
	)
	t.Cleanup(func() { _ = lp.Shutdown(context.Background()) })

	h := otelslog.NewHandler("test", otelslog.WithLoggerProvider(lp))
	l := slog.New(h)

	tp := sdktrace.NewTracerProvider()
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	ctx, span := tp.Tracer("test").Start(context.Background(), "op")
	defer span.End()

	l.InfoContext(ctx, "with-trace", "key", "val")

	_ = lp.ForceFlush(context.Background())

	exp.mu.Lock()
	defer exp.mu.Unlock()

	if len(exp.records) == 0 {
		t.Fatal("expected at least one log record")
	}

	r := exp.records[0]
	sc := trace.SpanContextFromContext(ctx)
	if r.TraceID() != sc.TraceID() {
		t.Errorf("trace_id mismatch: got %s, want %s", r.TraceID(), sc.TraceID())
	}
	if r.SpanID() != sc.SpanID() {
		t.Errorf("span_id mismatch: got %s, want %s", r.SpanID(), sc.SpanID())
	}
}

func TestOtelHandler_NoSpan_NoTrace(t *testing.T) {
	exp := &mockExporter{}
	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewSimpleProcessor(exp)),
	)
	t.Cleanup(func() { _ = lp.Shutdown(context.Background()) })

	h := otelslog.NewHandler("test", otelslog.WithLoggerProvider(lp))
	slog.New(h).Info("no-trace")

	_ = lp.ForceFlush(context.Background())

	exp.mu.Lock()
	defer exp.mu.Unlock()

	if len(exp.records) == 0 {
		t.Fatal("expected record")
	}
	if exp.records[0].TraceID().IsValid() {
		t.Error("expected invalid trace_id when no span in ctx")
	}
}
