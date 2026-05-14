package middleware

import (
	"net/http"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"go.opentelemetry.io/otel/metric/noop"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
	"github.com/Servora-Kit/servora/obs/telemetry"
)

func createTestMetrics() *telemetry.Metrics {
	meter := noop.NewMeterProvider().Meter("test")
	requests, _ := meter.Int64Counter("test_requests")
	seconds, _ := meter.Float64Histogram("test_seconds")
	return &telemetry.Metrics{
		Requests: requests,
		Seconds:  seconds,
		Handler:  http.NotFoundHandler(),
	}
}

func TestNewChainBuilder_BasicBuild(t *testing.T) {
	logger := log.DefaultLogger
	ms := NewChainBuilder(logger).Build()

	if len(ms) != 4 {
		t.Errorf("expected 4 middlewares (recovery, logging, ratelimit, validate), got %d", len(ms))
	}
}

func TestChainBuilder_WithTrace_Enabled(t *testing.T) {
	logger := log.DefaultLogger
	trace := &corev1.Trace{Endpoint: "http://jaeger:14268"}

	ms := NewChainBuilder(logger).WithTrace(trace).Build()

	if len(ms) != 5 {
		t.Errorf("expected 5 middlewares with tracing, got %d", len(ms))
	}
}

func TestChainBuilder_WithTrace_Skipped_NilTrace(t *testing.T) {
	logger := log.DefaultLogger

	ms := NewChainBuilder(logger).WithTrace(nil).Build()

	if len(ms) != 4 {
		t.Errorf("expected 4 middlewares without tracing (nil), got %d", len(ms))
	}
}

func TestChainBuilder_WithTrace_Skipped_EmptyEndpoint(t *testing.T) {
	logger := log.DefaultLogger
	trace := &corev1.Trace{Endpoint: ""}

	ms := NewChainBuilder(logger).WithTrace(trace).Build()

	if len(ms) != 4 {
		t.Errorf("expected 4 middlewares without tracing (empty endpoint), got %d", len(ms))
	}
}

func TestChainBuilder_WithMetrics_Enabled(t *testing.T) {
	logger := log.DefaultLogger
	mtc := createTestMetrics()

	ms := NewChainBuilder(logger).WithMetrics(mtc).Build()

	if len(ms) != 5 {
		t.Errorf("expected 5 middlewares with metrics, got %d", len(ms))
	}
}

func TestChainBuilder_WithMetrics_Skipped(t *testing.T) {
	logger := log.DefaultLogger

	ms := NewChainBuilder(logger).WithMetrics(nil).Build()

	if len(ms) != 4 {
		t.Errorf("expected 4 middlewares without metrics (nil), got %d", len(ms))
	}
}

func TestChainBuilder_WithoutRateLimit(t *testing.T) {
	logger := log.DefaultLogger

	ms := NewChainBuilder(logger).WithoutRateLimit().Build()

	if len(ms) != 3 {
		t.Errorf("expected 3 middlewares without ratelimit, got %d", len(ms))
	}
}

func TestChainBuilder_FullChain(t *testing.T) {
	logger := log.DefaultLogger
	trace := &corev1.Trace{Endpoint: "http://jaeger:14268"}
	mtc := createTestMetrics()

	ms := NewChainBuilder(logger).
		WithTrace(trace).
		WithMetrics(mtc).
		Build()

	if len(ms) != 6 {
		t.Errorf("expected 6 middlewares in full chain, got %d", len(ms))
	}
}

func TestChainBuilder_MinimalChain(t *testing.T) {
	logger := log.DefaultLogger

	ms := NewChainBuilder(logger).
		WithoutRateLimit().
		Build()

	if len(ms) != 3 {
		t.Errorf("expected 3 middlewares in minimal chain (recovery, logging, validate), got %d", len(ms))
	}
}

func TestChainBuilder_Appendable(t *testing.T) {
	logger := log.DefaultLogger

	ms := NewChainBuilder(logger).Build()
	originalLen := len(ms)

	ms = append(ms, nil)
	if len(ms) != originalLen+1 {
		t.Errorf("expected slice to be appendable")
	}
}

func TestChainBuilder_BuildIdempotent(t *testing.T) {
	logger := log.DefaultLogger
	b := NewChainBuilder(logger)
	ms1 := b.Build()
	ms2 := b.Build()
	if &ms1[0] == &ms2[0] {
		t.Fatalf("Build should return new slice each call")
	}
	if len(ms1) != len(ms2) {
		t.Fatalf("Build outputs should have equal length: %d vs %d", len(ms1), len(ms2))
	}
}
