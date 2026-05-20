package middleware

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
	"github.com/Servora-Kit/servora/obs/telemetry"
	"github.com/Servora-Kit/servora/security/authn/jwt"
	"log/slog"
	"github.com/go-kratos/kratos/v2/transport"
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
	ms := NewChainBuilder(slog.Default()).Build()
	if len(ms) != 3 {
		t.Errorf("expected 3 middlewares (recovery,logging,circuit), got %d", len(ms))
	}
}

func TestChainBuilder_WithTrace_Enabled(t *testing.T) {
	trace := &corev1.Trace{Endpoint: "http://otel:4317"}
	ms := NewChainBuilder(slog.Default()).WithTrace(trace).Build()
	if len(ms) != 4 {
		t.Errorf("expected 4 middlewares with tracing, got %d", len(ms))
	}
}

func TestChainBuilder_WithMetrics_Enabled(t *testing.T) {
	mtc := createTestMetrics()
	ms := NewChainBuilder(slog.Default()).WithMetrics(mtc).Build()
	if len(ms) != 4 {
		t.Errorf("expected 4 middlewares with metrics, got %d", len(ms))
	}
}

func TestChainBuilder_WithoutCircuitBreaker(t *testing.T) {
	ms := NewChainBuilder(slog.Default()).WithoutCircuitBreaker().Build()
	if len(ms) != 2 {
		t.Errorf("expected 2 middlewares without circuitbreaker, got %d", len(ms))
	}
}

// ============================================================================
// Anti-regression: default chain MUST NOT include jwt token propagation.
//
// The legacy `TokenPropagation()` middleware (and the ctx-key channel it read
// from in transport/server/middleware/token.go) baked a jwt-shaped contract
// into the engine-agnostic transport layer. After the authn engine-agnostic
// refactor, jwt token propagation lives in `security/authn/jwt.Client()` and
// must be opted in by callers — see jwt/client.go godoc.
//
// Two complementary guards below:
//   1. structural (source-grep): chain.go MUST NOT reference TokenPropagation
//      or the deleted authn.go file.
//   2. behavioral: route a jwt-stamped ctx through every middleware in the
//      default-built chain and assert no outbound Authorization header is set.
// ============================================================================

// TestChain_DoesNotReferenceTokenPropagation is a structural guard: read the
// source of chain.go and assert there is no `TokenPropagation` reference and
// no leftover import of the deleted client/middleware/authn.go symbols.
func TestChain_DoesNotReferenceTokenPropagation(t *testing.T) {
	src, err := os.ReadFile("chain.go")
	if err != nil {
		t.Fatalf("read chain.go: %v", err)
	}
	body := string(src)
	if strings.Contains(body, "TokenPropagation") {
		t.Error("chain.go MUST NOT reference TokenPropagation after refactor (jwt token propagation is opt-in via security/authn/jwt.Client())")
	}
	if strings.Contains(body, "svrmw") {
		t.Error("chain.go MUST NOT import or reference svrmw (transport server middleware) after refactor")
	}
}

// fakeChainTransport is a minimal client-side Transporter that records every
// header set during the middleware chain pass-through. We assert across the
// recorded headers that no `Authorization` header is set by any middleware in
// the default-built chain.
type fakeChainTransport struct {
	headers map[string]string
}

func (f *fakeChainTransport) Kind() transport.Kind            { return transport.KindHTTP }
func (f *fakeChainTransport) Endpoint() string                { return "" }
func (f *fakeChainTransport) Operation() string               { return "" }
func (f *fakeChainTransport) RequestHeader() transport.Header { return &fakeChainHeader{f.headers} }
func (f *fakeChainTransport) ReplyHeader() transport.Header   { return &fakeChainHeader{} }

type fakeChainHeader struct {
	m map[string]string
}

func (h *fakeChainHeader) Get(key string) string    { return h.m[key] }
func (h *fakeChainHeader) Set(key, value string)    { h.m[key] = value }
func (h *fakeChainHeader) Add(_ string, _ string)   {}
func (h *fakeChainHeader) Keys() []string           { return nil }
func (h *fakeChainHeader) Values(_ string) []string { return nil }

// TestChain_DefaultDoesNotPropagateJWTToken is the behavioral guard: when a
// jwt-stamped ctx is routed through the full default-built chain, no
// middleware in the chain may set the outbound Authorization header.
//
// Wiring detail: middleware.Middleware is a higher-order wrapper, so we walk
// the slice and compose them around a leaf handler that does nothing. The
// outbound transport is attached to the ctx so that any middleware which
// *would* attempt header writes (the legacy TokenPropagation) succeeds in
// reaching `tr.RequestHeader().Set("Authorization", …)` — its ABSENCE in the
// chain is what we assert.
func TestChain_DefaultDoesNotPropagateJWTToken(t *testing.T) {
	tr := &fakeChainTransport{headers: map[string]string{}}
	ctx := transport.NewClientContext(context.Background(), tr)
	ctx = jwt.WithToken(ctx, "should-not-be-propagated")

	ms := NewChainBuilder(slog.Default()).Build()

	// Compose the chain right-to-left around a leaf handler.
	handler := func(_ context.Context, _ any) (any, error) {
		return "ok", nil
	}
	for i := len(ms) - 1; i >= 0; i-- {
		handler = ms[i](handler)
	}

	if _, err := handler(ctx, struct{}{}); err != nil {
		t.Fatalf("default chain handler returned unexpected error: %v", err)
	}

	if v := tr.headers["Authorization"]; v != "" {
		t.Errorf("default chain MUST NOT set Authorization header, got %q", v)
	}
}
