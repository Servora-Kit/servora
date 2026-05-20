package telemetry

import (
	"log/slog"
	"net/http"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
	"github.com/go-kratos/kratos/v2/middleware/metrics"
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

func NewMetrics(c *corev1.Metrics, app *corev1.App, l *slog.Logger) (*Metrics, error) {
	if c == nil || !c.Enable {
		if l != nil {
			l.With("scope", "metrics/telemetry").Info("metrics config is empty, skip metrics init")
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

	requests, err := metrics.DefaultRequestsCounter(meter, metrics.DefaultServerRequestsCounterName)
	if err != nil {
		return nil, err
	}

	seconds, err := metrics.DefaultSecondsHistogram(meter, metrics.DefaultServerSecondsHistogramName)
	if err != nil {
		return nil, err
	}

	return &Metrics{
		Requests: requests,
		Seconds:  seconds,
		Handler:  promhttp.Handler(),
	}, nil
}
