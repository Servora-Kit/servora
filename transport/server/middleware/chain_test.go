package middleware

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	kratosmw "github.com/go-kratos/kratos/v2/middleware"
	"go.opentelemetry.io/otel/metric/noop"

	auditpb "github.com/Servora-Kit/servora/api/gen/go/servora/audit/v1"
	"github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/obs/audit"
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
	trace := &conf.Trace{Endpoint: "http://jaeger:14268"}

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
	trace := &conf.Trace{Endpoint: ""}

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
	trace := &conf.Trace{Endpoint: "http://jaeger:14268"}
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

// --- WithAudit unit tests ---

func TestChainBuilder_WithoutAudit_NoCollector(t *testing.T) {
	logger := log.DefaultLogger
	ms := NewChainBuilder(logger).Build()
	// baseline = recovery + logging + ratelimit + validate = 4
	if got := len(ms); got != 4 {
		t.Fatalf("expected 4 middlewares (no audit), got %d", got)
	}
}

func TestChainBuilder_WithAudit_AppendsCollectorLast(t *testing.T) {
	logger := log.DefaultLogger
	rec := audit.NewRecorder(audit.NewNoopEmitter(), "test")
	ms := NewChainBuilder(logger).WithAudit(rec).Build()
	if got := len(ms); got != 5 {
		t.Fatalf("expected 5 middlewares (with audit), got %d", got)
	}
	if ms[len(ms)-1] == nil {
		t.Fatalf("last middleware should be audit.Collector, got nil")
	}
}

func TestChainBuilder_WithAudit_Nil_Skipped(t *testing.T) {
	logger := log.DefaultLogger
	ms := NewChainBuilder(logger).WithAudit(nil).Build()
	if got := len(ms); got != 4 {
		t.Fatalf("WithAudit(nil) should skip; expected 4 got %d", got)
	}
}

func TestChainBuilder_WithAudit_NilThenRec(t *testing.T) {
	logger := log.DefaultLogger
	rec := audit.NewRecorder(audit.NewNoopEmitter(), "test")
	ms := NewChainBuilder(logger).WithAudit(nil).WithAudit(rec).Build()
	if got := len(ms); got != 5 {
		t.Fatalf("WithAudit(nil).WithAudit(rec) should retain rec; expected 5 got %d", got)
	}
}

func TestChainBuilder_WithAudit_LastWriteWins(t *testing.T) {
	logger := log.DefaultLogger
	rec1 := audit.NewRecorder(audit.NewNoopEmitter(), "a")
	rec2 := audit.NewRecorder(audit.NewNoopEmitter(), "b")
	b := NewChainBuilder(logger).WithAudit(rec1).WithAudit(rec2)
	if b.auditRecorder != rec2 {
		t.Fatalf("expected last rec to win, got first")
	}
}

func TestChainBuilder_BuildIdempotent(t *testing.T) {
	logger := log.DefaultLogger
	rec := audit.NewRecorder(audit.NewNoopEmitter(), "test")
	b := NewChainBuilder(logger).WithAudit(rec)
	ms1 := b.Build()
	ms2 := b.Build()
	// slice header 不等 = 每次 Build 返回独立切片（独立底层数组）。
	// 注：spec scenario 还提到"末尾 audit middleware 是不同的 closure 实例"，
	// 但 Go 中 reflect.Value.Pointer() 对 func 返回的是代码地址而非闭包数据，
	// 两个共享同一函数字面量的闭包 code pointer 必然相等；slice header 不等
	// 已是 Go 范畴内对"独立实例"最强的可断言形式。
	if &ms1[0] == &ms2[0] {
		t.Fatalf("Build should return new slice each call")
	}
	if len(ms1) != len(ms2) {
		t.Fatalf("Build outputs should have equal length: %d vs %d", len(ms1), len(ms2))
	}
}

func TestChainBuilder_WithAudit_OptionPassThrough(t *testing.T) {
	logger := log.DefaultLogger
	rec := audit.NewRecorder(audit.NewNoopEmitter(), "test")
	b := NewChainBuilder(logger).WithAudit(rec, audit.WithSpanEvents(false))
	if got := len(b.auditOpts); got != 1 {
		t.Fatalf("expected 1 option, got %d", got)
	}
}

// --- WithAudit E2E tests ---

// captureEmitter is an in-memory audit Emitter for E2E behavioural assertions.
type captureEmitter struct {
	mu     sync.Mutex
	events []*auditpb.AuditEvent
}

func (c *captureEmitter) Emit(_ context.Context, evt *auditpb.AuditEvent) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, evt)
	return nil
}

func (c *captureEmitter) Close() error { return nil }

// failAuthnMiddleware simulates an authn middleware that writes an authn-failure
// detail into ctx and short-circuits with an error. Used to validate that
// audit.Collector — appended at the tail by WithAudit — still emits the
// AUTHN_RESULT event from its LIFO post-phase.
func failAuthnMiddleware(reason string) kratosmw.Middleware {
	return func(next kratosmw.Handler) kratosmw.Handler {
		return func(ctx context.Context, _ any) (any, error) {
			// 返回的 ctx 故意丢弃——audit 用 mutable holder pattern，
			// `WithAuthnResult` 通过 ctx 中的 holder 指针就地 mutate；
			// 外层 Collector 装入的同一份 holder 在 post-phase 能读到。
			audit.WithAuthnResult(ctx, &auditpb.AuthnDetail{
				Method:        "test",
				Success:       false,
				FailureReason: reason,
			})
			return nil, errors.New(reason)
		}
	}
}

func TestChainBuilder_E2E_WithAudit_EmitsOnAuthnFailure(t *testing.T) {
	logger := log.DefaultLogger
	cap := &captureEmitter{}
	rec := audit.NewRecorder(cap, "test")

	ms := NewChainBuilder(logger).WithoutRateLimit().WithAudit(rec).Build()
	ms = append(ms, failAuthnMiddleware("invalid token"))

	handler := func(_ context.Context, _ any) (any, error) { return "ok", nil }
	chain := kratosmw.Chain(ms...)(handler)

	_, err := chain(context.Background(), nil)
	if err == nil {
		t.Fatal("expected handler error from authn fail")
	}

	cap.mu.Lock()
	defer cap.mu.Unlock()
	if len(cap.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(cap.events))
	}
	evt := cap.events[0]
	if evt.GetEventType() != auditpb.AuditEventType_AUDIT_EVENT_TYPE_AUTHN_RESULT {
		t.Fatalf("expected AUTHN_RESULT, got %v", evt.GetEventType())
	}
	if evt.GetResult().GetSuccess() {
		t.Fatal("expected Result.Success=false")
	}
	if got := evt.GetResult().GetErrorMessage(); got != "invalid token" {
		t.Fatalf("expected error_message=%q, got %q", "invalid token", got)
	}
}
