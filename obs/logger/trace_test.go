package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	slogzap "github.com/samber/slog-zap/v2"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func validSpanCtx(t *testing.T) (context.Context, trace.TraceID, trace.SpanID) {
	t.Helper()
	tid, err := trace.TraceIDFromHex("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("TraceIDFromHex: %v", err)
	}
	sid, err := trace.SpanIDFromHex("0123456789abcdef")
	if err != nil {
		t.Fatalf("SpanIDFromHex: %v", err)
	}
	sc := trace.NewSpanContext(trace.SpanContextConfig{TraceID: tid, SpanID: sid})
	return trace.ContextWithSpanContext(context.Background(), sc), tid, sid
}

func TestTraceAttrs_ValidSpan(t *testing.T) {
	ctx, tid, sid := validSpanCtx(t)
	attrs := traceAttrs(ctx)
	if len(attrs) != 2 {
		t.Fatalf("expected 2 attrs, got %d: %v", len(attrs), attrs)
	}
	got := map[string]string{}
	for _, a := range attrs {
		got[a.Key] = a.Value.String()
	}
	if got[TraceIDKey] != tid.String() {
		t.Fatalf("trace_id = %q, want %q", got[TraceIDKey], tid.String())
	}
	if got[SpanIDKey] != sid.String() {
		t.Fatalf("span_id = %q, want %q", got[SpanIDKey], sid.String())
	}
}

func TestTraceAttrs_NoSpan(t *testing.T) {
	if attrs := traceAttrs(context.Background()); len(attrs) != 0 {
		t.Fatalf("expected no attrs without span, got %v", attrs)
	}
}

func bufZap(buf *bytes.Buffer) *zap.Logger {
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(zapcore.EncoderConfig{
			MessageKey:  "msg",
			LevelKey:    "level",
			EncodeLevel: zapcore.CapitalLevelEncoder,
		}),
		zapcore.AddSync(buf),
		zapcore.DebugLevel,
	)
	return zap.New(core)
}

func TestNewHandler_InjectsTraceEndToEnd(t *testing.T) {
	var buf bytes.Buffer
	z := bufZap(&buf)
	h := slogzap.Option{
		Logger:          z,
		AttrFromContext: []func(context.Context) []slog.Attr{traceAttrs},
	}.NewZapHandler()

	ctx, tid, sid := validSpanCtx(t)
	slog.New(h).InfoContext(ctx, "hi")
	if err := z.Sync(); err != nil {
		t.Fatalf("zap sync: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("unmarshal %q: %v", buf.String(), err)
	}
	if m[TraceIDKey] != tid.String() {
		t.Fatalf("trace_id = %v, want %q (log=%s)", m[TraceIDKey], tid.String(), buf.String())
	}
	if m[SpanIDKey] != sid.String() {
		t.Fatalf("span_id = %v, want %q (log=%s)", m[SpanIDKey], sid.String(), buf.String())
	}
}

func TestNewHandler_NoSpanEndToEnd(t *testing.T) {
	var buf bytes.Buffer
	z := bufZap(&buf)
	h := slogzap.Option{
		Logger:          z,
		AttrFromContext: []func(context.Context) []slog.Attr{traceAttrs},
	}.NewZapHandler()

	slog.New(h).InfoContext(context.Background(), "hi")
	if err := z.Sync(); err != nil {
		t.Fatalf("zap sync: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("unmarshal %q: %v", buf.String(), err)
	}
	if _, ok := m[TraceIDKey]; ok {
		t.Fatalf("trace_id should be absent without span (log=%s)", buf.String())
	}
	if _, ok := m[SpanIDKey]; ok {
		t.Fatalf("span_id should be absent without span (log=%s)", buf.String())
	}
}
