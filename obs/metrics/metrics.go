package metrics

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
	kratosmetrics "github.com/go-kratos/kratos/contrib/otel/v3/metrics"
	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

const (
	meterName             = "github.com/Servora-Kit/servora/obs/metrics"
	histogramBucketSuffix = "_bucket"
)

type Metrics struct {
	ServerRequests metric.Int64Counter
	ServerSeconds  metric.Float64Histogram
	ClientRequests metric.Int64Counter
	ClientSeconds  metric.Float64Histogram
	Handler        http.Handler

	provider *sdkmetric.MeterProvider
	close    sync.Once
	closeErr error
}

func New(obs *corev1.Observability, app *corev1.App, l *slog.Logger) (*Metrics, func(), error) {
	c := obs.GetMetrics()
	if c == nil || !c.Enable {
		if l != nil {
			l.With("scope", "obs/metrics").Info("metrics config is empty, skip metrics init")
		}
		return nil, func() {}, nil
	}

	reg := promclient.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	exporter, err := otelprom.New(otelprom.WithRegisterer(reg))
	if err != nil {
		return nil, nil, err
	}
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(newResource(app)),
		sdkmetric.WithReader(exporter),
		sdkmetric.WithView(
			kratosmetrics.DefaultSecondsHistogramView(histogramInstrumentName(kratosmetrics.DefaultServerSecondsHistogramName)),
			kratosmetrics.DefaultSecondsHistogramView(histogramInstrumentName(kratosmetrics.DefaultClientSecondsHistogramName)),
		),
	)
	otel.SetMeterProvider(provider)

	meter := provider.Meter(meterName)

	serverRequests, err := kratosmetrics.DefaultRequestsCounter(meter, kratosmetrics.DefaultServerRequestsCounterName)
	if err != nil {
		_ = provider.Shutdown(context.Background())
		return nil, nil, err
	}

	serverSeconds, err := kratosmetrics.DefaultSecondsHistogram(meter, histogramInstrumentName(kratosmetrics.DefaultServerSecondsHistogramName))
	if err != nil {
		_ = provider.Shutdown(context.Background())
		return nil, nil, err
	}

	clientRequests, err := kratosmetrics.DefaultRequestsCounter(meter, kratosmetrics.DefaultClientRequestsCounterName)
	if err != nil {
		_ = provider.Shutdown(context.Background())
		return nil, nil, err
	}

	clientSeconds, err := kratosmetrics.DefaultSecondsHistogram(meter, histogramInstrumentName(kratosmetrics.DefaultClientSecondsHistogramName))
	if err != nil {
		_ = provider.Shutdown(context.Background())
		return nil, nil, err
	}

	m := &Metrics{
		ServerRequests: serverRequests,
		ServerSeconds:  serverSeconds,
		ClientRequests: clientRequests,
		ClientSeconds:  clientSeconds,
		Handler:        promhttp.HandlerFor(reg, promhttp.HandlerOpts{}),
		provider:       provider,
	}

	return m, func() {
		_ = m.Close(context.Background())
	}, nil
}

func (m *Metrics) Meter(name string, opts ...metric.MeterOption) metric.Meter {
	if m == nil || m.provider == nil {
		return noop.NewMeterProvider().Meter(name, opts...)
	}
	return m.provider.Meter(name, opts...)
}

func (m *Metrics) Close(ctx context.Context) error {
	if m == nil || m.provider == nil {
		return nil
	}
	m.close.Do(func() {
		m.closeErr = m.provider.Shutdown(ctx)
	})
	return m.closeErr
}

func newResource(app *corev1.App) *resource.Resource {
	serviceName := strings.TrimSpace(app.GetName())
	if serviceName == "" {
		serviceName = "unknown-service"
	}

	attrs := []attribute.KeyValue{
		semconv.ServiceName(serviceName),
	}
	if version := strings.TrimSpace(app.GetVersion()); version != "" {
		attrs = append(attrs, semconv.ServiceVersion(version))
	}
	if env := strings.TrimSpace(app.GetEnv()); env != "" {
		attrs = append(attrs, attribute.String("deployment.environment.name", env))
	}

	return resource.NewSchemaless(attrs...)
}

func histogramInstrumentName(name string) string {
	return strings.TrimSuffix(name, histogramBucketSuffix)
}
