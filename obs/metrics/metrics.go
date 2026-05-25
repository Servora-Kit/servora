package metrics

import (
	"log/slog"
	"net/http"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
	kratosmetrics "github.com/go-kratos/kratos/v2/middleware/metrics"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

type Metrics struct {
	Requests metric.Int64Counter
	Seconds  metric.Float64Histogram
	Handler  http.Handler
}

func New(obs *corev1.Observability, app *corev1.App, l *slog.Logger) (*Metrics, error) {
	c := obs.GetMetrics()
	if c == nil || !c.Enable {
		if l != nil {
			l.With("scope", "obs/metrics").Info("metrics config is empty, skip metrics init")
		}
		return nil, nil
	}

	exporter, err := prometheus.New()
	if err != nil {
		return nil, err
	}
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter))

	meterName := app.GetName()
	if meterName == "" {
		meterName = "unknown-service"
	}
	meter := provider.Meter(meterName)

	requests, err := kratosmetrics.DefaultRequestsCounter(meter, kratosmetrics.DefaultServerRequestsCounterName)
	if err != nil {
		return nil, err
	}

	seconds, err := kratosmetrics.DefaultSecondsHistogram(meter, kratosmetrics.DefaultServerSecondsHistogramName)
	if err != nil {
		return nil, err
	}

	return &Metrics{
		Requests: requests,
		Seconds:  seconds,
		Handler:  promhttp.Handler(),
	}, nil
}
