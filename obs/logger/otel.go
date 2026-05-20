package logger

import (
	"context"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	sdklog "go.opentelemetry.io/otel/sdk/log"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
)

func buildOtelHandler(cfg *corev1.Log_OtelBackend, _ slog.Level) (slog.Handler, func(context.Context) error) {
	if cfg == nil || cfg.GetEndpoint() == "" {
		return nil, nil
	}

	ctx := context.Background()
	exp, err := newOtelExporter(ctx, cfg)
	if err != nil {
		slog.Default().Error("failed to create otel log exporter", "err", err)
		return nil, nil
	}

	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exp)),
	)

	h := otelslog.NewHandler("", otelslog.WithLoggerProvider(lp))
	closer := func(ctx context.Context) error {
		return lp.Shutdown(ctx)
	}
	return h, closer
}

func newOtelExporter(ctx context.Context, cfg *corev1.Log_OtelBackend) (sdklog.Exporter, error) {
	endpoint := cfg.GetEndpoint()
	insecure := cfg.GetInsecure()

	switch cfg.GetProtocol() {
	case corev1.Log_OTEL_PROTOCOL_HTTP_PROTOBUF:
		opts := []otlploghttp.Option{otlploghttp.WithEndpoint(endpoint)}
		if insecure {
			opts = append(opts, otlploghttp.WithInsecure())
		}
		return otlploghttp.New(ctx, opts...)
	default:
		opts := []otlploggrpc.Option{otlploggrpc.WithEndpoint(endpoint)}
		if insecure {
			opts = append(opts, otlploggrpc.WithInsecure())
		}
		exp, err := otlploggrpc.New(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("otlploggrpc: %w", err)
		}
		return exp, nil
	}
}
