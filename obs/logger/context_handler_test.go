package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestContextHandler_InjectsTraceSpan(t *testing.T) {
	var buf bytes.Buffer
	h := newContextHandler(slog.NewJSONHandler(&buf, nil))
	tid, _ := trace.TraceIDFromHex("0123456789abcdef0123456789abcdef")
	sid, _ := trace.SpanIDFromHex("0123456789abcdef")
	sc := trace.NewSpanContext(trace.SpanContextConfig{TraceID: tid, SpanID: sid})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)
	slog.New(h).InfoContext(ctx, "hi")
	var m map[string]any
	_ = json.Unmarshal(buf.Bytes(), &m)
	if m["trace_id"] != tid.String() || m["span_id"] != sid.String() {
		t.Fatalf("got trace=%v span=%v", m["trace_id"], m["span_id"])
	}
}

func TestContextHandler_NoSpanIsSafe(t *testing.T) {
	var buf bytes.Buffer
	slog.New(newContextHandler(slog.NewJSONHandler(&buf, nil))).
		InfoContext(context.Background(), "hi")
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["trace_id"]; ok {
		t.Fatal("trace_id should be absent without span")
	}
}
