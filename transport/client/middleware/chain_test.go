package middleware

import (
	"net/http"
	"testing"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/obs/telemetry"
	"github.com/go-kratos/kratos/v2/log"
	"go.opentelemetry.io/otel/metric/noop"
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
	ms := NewChainBuilder(log.DefaultLogger).Build()
	if len(ms) != 4 {
		t.Errorf("expected 4 middlewares (recovery,tracing?,logging,circuit,token), got %d", len(ms))
	}
}

func TestChainBuilder_WithTrace_Enabled(t *testing.T) {
	trace := &conf.Trace{Endpoint: "http://otel:4317"}
	ms := NewChainBuilder(log.DefaultLogger).WithTrace(trace).Build()
	if len(ms) != 5 {
		t.Errorf("expected 5 middlewares with tracing, got %d", len(ms))
	}
}

func TestChainBuilder_WithMetrics_Enabled(t *testing.T) {
	mtc := createTestMetrics()
	ms := NewChainBuilder(log.DefaultLogger).WithMetrics(mtc).Build()
	if len(ms) != 5 {
		t.Errorf("expected 5 middlewares with metrics, got %d", len(ms))
	}
}

func TestChainBuilder_WithoutCircuitBreaker(t *testing.T) {
	ms := NewChainBuilder(log.DefaultLogger).WithoutCircuitBreaker().Build()
	if len(ms) != 3 {
		t.Errorf("expected 3 middlewares without circuitbreaker, got %d", len(ms))
	}
}
