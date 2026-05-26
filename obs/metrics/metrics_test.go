package metrics

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
	promclient "github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
)

func TestNew_Disabled(t *testing.T) {
	m, cleanup, err := New(nil, &corev1.App{Name: "disabled"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("New disabled error = %v", err)
	}
	if m != nil {
		t.Fatalf("New disabled metrics = %#v, want nil", m)
	}
	if cleanup == nil {
		t.Fatal("New disabled cleanup is nil")
	}
	cleanup()
}

func TestNew_HandlerExposesRuntimeAndCustomMetrics(t *testing.T) {
	restoreMeterProvider(t)

	m, cleanup, err := New(
		&corev1.Observability{Metrics: &corev1.Metrics{Enable: true}},
		&corev1.App{Name: "metrics-test", Env: "test", Version: "v0.0.0"},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	if err != nil {
		t.Fatalf("New enabled error = %v", err)
	}
	t.Cleanup(cleanup)
	if m == nil {
		t.Fatal("New enabled metrics is nil")
	}
	if m.Handler == nil {
		t.Fatal("New enabled handler is nil")
	}

	meter := m.Meter("github.com/Servora-Kit/servora/obs/metrics/test")
	counter, err := meter.Int64Counter("servora_custom_events")
	if err != nil {
		t.Fatalf("create custom counter: %v", err)
	}
	counter.Add(context.Background(), 3)

	body := scrape(t, m.Handler)
	assertContains(t, body, "servora_custom_events")
	assertContains(t, body, "go_goroutines")
	assertContains(t, body, "target_info")
	assertContains(t, body, `service_name="metrics-test"`)
}

func TestNew_HandlerExposesServerAndClientInstruments(t *testing.T) {
	restoreMeterProvider(t)

	m, cleanup, err := New(
		&corev1.Observability{Metrics: &corev1.Metrics{Enable: true}},
		&corev1.App{Name: "instrument-test"},
		nil,
	)
	if err != nil {
		t.Fatalf("New enabled error = %v", err)
	}
	t.Cleanup(cleanup)

	ctx := context.Background()
	m.ServerRequests.Add(ctx, 1)
	m.ServerSeconds.Record(ctx, 0.01)
	m.ClientRequests.Add(ctx, 1)
	m.ClientSeconds.Record(ctx, 0.02)

	body := scrape(t, m.Handler)
	assertContains(t, body, "server_requests_code_total")
	assertContains(t, body, "server_requests_seconds_bucket")
	assertContains(t, body, "client_requests_code_total")
	assertContains(t, body, "client_requests_seconds_bucket")
	if strings.Contains(body, "requests_seconds_bucket_bucket") {
		t.Fatalf("histogram name has duplicate bucket suffix:\n%s", body)
	}
}

func TestNew_GlobalMeterProviderExportsThroughHandler(t *testing.T) {
	restoreMeterProvider(t)

	m, cleanup, err := New(
		&corev1.Observability{Metrics: &corev1.Metrics{Enable: true}},
		&corev1.App{Name: "global-provider-test"},
		nil,
	)
	if err != nil {
		t.Fatalf("New enabled error = %v", err)
	}
	t.Cleanup(cleanup)

	counter, err := otel.Meter("github.com/example/thirdparty").Int64Counter("servora_global_provider_events")
	if err != nil {
		t.Fatalf("create global-provider counter: %v", err)
	}
	counter.Add(context.Background(), 1)

	body := scrape(t, m.Handler)
	assertContains(t, body, "servora_global_provider_events")
}

func TestNew_DefaultPrometheusRegistryDoesNotLeak(t *testing.T) {
	restoreMeterProvider(t)

	name := uniqueMetricName(t, "servora_default_registry_leak")
	c := promclient.NewCounter(promclient.CounterOpts{
		Name: name,
		Help: "metric registered only on the Prometheus default registry",
	})
	if err := promclient.DefaultRegisterer.Register(c); err != nil {
		t.Fatalf("register default metric: %v", err)
	}
	t.Cleanup(func() {
		promclient.DefaultRegisterer.Unregister(c)
	})
	c.Inc()

	m, cleanup, err := New(
		&corev1.Observability{Metrics: &corev1.Metrics{Enable: true}},
		&corev1.App{Name: "registry-test"},
		nil,
	)
	if err != nil {
		t.Fatalf("New enabled error = %v", err)
	}
	t.Cleanup(cleanup)

	body := scrape(t, m.Handler)
	if strings.Contains(body, name) {
		t.Fatalf("Servora metrics handler leaked default registry metric %q:\n%s", name, body)
	}
}

func TestMetrics_CloseIsIdempotent(t *testing.T) {
	restoreMeterProvider(t)

	m, cleanup, err := New(
		&corev1.Observability{Metrics: &corev1.Metrics{Enable: true}},
		&corev1.App{Name: "close-test"},
		nil,
	)
	if err != nil {
		t.Fatalf("New enabled error = %v", err)
	}
	t.Cleanup(cleanup)

	if err := m.Close(context.Background()); err != nil {
		t.Fatalf("first Close error = %v", err)
	}
	if err := m.Close(context.Background()); err != nil {
		t.Fatalf("second Close error = %v", err)
	}
}

func TestMetrics_NilMeterIsNoop(t *testing.T) {
	var m *Metrics
	if meter := m.Meter("github.com/Servora-Kit/servora/obs/metrics/test"); meter == nil {
		t.Fatal("nil Metrics returned nil Meter")
	}
}

func scrape(t *testing.T, h http.Handler) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("scrape status = %d, body:\n%s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

func assertContains(t *testing.T, body, want string) {
	t.Helper()
	if !strings.Contains(body, want) {
		t.Fatalf("metrics output missing %q:\n%s", want, body)
	}
}

func restoreMeterProvider(t *testing.T) {
	t.Helper()
	prev := otel.GetMeterProvider()
	t.Cleanup(func() {
		otel.SetMeterProvider(prev)
	})
}

func uniqueMetricName(t *testing.T, prefix string) string {
	t.Helper()
	replacer := strings.NewReplacer("/", "_", "-", "_", " ", "_")
	return replacer.Replace(prefix+"_"+t.Name()) + "_" + strconv.FormatInt(time.Now().UnixNano(), 10)
}
