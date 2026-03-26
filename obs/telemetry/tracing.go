package telemetry

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
	"time"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"google.golang.org/grpc/credentials"
)

const (
	shutdownTimeout          = 5 * time.Second
	defaultProdSamplingRatio = 0.1
	defaultDevSamplingRatio  = 1.0
)

type traceRuntimeConfig struct {
	endpoint      string
	insecure      bool
	samplingRatio float64
	caPath        string
}

// InitTracerProvider 初始化 OpenTelemetry Trace Provider，并返回关闭回调。
func InitTracerProvider(c *conf.Trace, serviceName, env string) (func(), error) {
	runtimeCfg := resolveTraceRuntimeConfig(c, env)
	if runtimeCfg.endpoint == "" {
		return func() {}, nil
	}
	if strings.TrimSpace(serviceName) == "" {
		serviceName = "unknown.service"
	}
	if strings.TrimSpace(env) == "" {
		env = "unknown"
	}

	exporterOpts, err := newTraceExporterOptions(runtimeCfg)
	if err != nil {
		return nil, err
	}

	exporter, err := otlptracegrpc.New(context.Background(), exporterOpts...)
	if err != nil {
		return nil, err
	}

	tp := tracesdk.NewTracerProvider(
		tracesdk.WithSampler(tracesdk.ParentBased(tracesdk.TraceIDRatioBased(runtimeCfg.samplingRatio))),
		tracesdk.WithBatcher(exporter),
		tracesdk.WithResource(resource.NewSchemaless(
			semconv.ServiceName(serviceName),
			attribute.String("deployment.environment.name", env),
			attribute.String("exporter", "otlp"),
		)),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		_ = tp.Shutdown(ctx)
	}

	return cleanup, nil
}

// resolveTraceRuntimeConfig 将配置文件与环境默认值归一化为运行时追踪配置。
func resolveTraceRuntimeConfig(c *conf.Trace, env string) traceRuntimeConfig {
	runtimeCfg := traceRuntimeConfig{
		samplingRatio: defaultTraceSamplingRatio(env),
	}
	if c == nil {
		return runtimeCfg
	}

	runtimeCfg.endpoint = strings.TrimSpace(c.GetEndpoint())
	runtimeCfg.insecure = c.GetInsecure()
	runtimeCfg.caPath = strings.TrimSpace(c.GetCaPath())

	if samplingRatio := c.GetSamplingRatio(); samplingRatio > 0 && samplingRatio <= 1 {
		runtimeCfg.samplingRatio = samplingRatio
	}

	return runtimeCfg
}

// defaultTraceSamplingRatio 根据部署环境返回默认采样率。
func defaultTraceSamplingRatio(env string) float64 {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "dev", "development", "local", "test":
		return defaultDevSamplingRatio
	default:
		return defaultProdSamplingRatio
	}
}

// newTraceExporterOptions 构造 OTLP Trace 导出器选项，并校验传输层配置是否冲突。
func newTraceExporterOptions(runtimeCfg traceRuntimeConfig) ([]otlptracegrpc.Option, error) {
	if runtimeCfg.endpoint == "" {
		return nil, fmt.Errorf("trace endpoint is required")
	}
	if runtimeCfg.insecure && runtimeCfg.caPath != "" {
		return nil, fmt.Errorf("trace config cannot enable insecure transport and custom ca_path at the same time")
	}

	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(runtimeCfg.endpoint),
	}

	if runtimeCfg.insecure {
		return append(opts, otlptracegrpc.WithInsecure()), nil
	}

	if runtimeCfg.caPath != "" {
		tlsCfg, err := loadTraceTLSConfig(runtimeCfg.caPath)
		if err != nil {
			return nil, err
		}
		opts = append(opts, otlptracegrpc.WithTLSCredentials(credentials.NewTLS(tlsCfg)))
	}

	return opts, nil
}

// loadTraceTLSConfig 从自定义 CA 文件加载追踪导出器的 TLS 根证书配置。
func loadTraceTLSConfig(caPath string) (*tls.Config, error) {
	caPEM, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("read trace ca file: %w", err)
	}

	rootCAs := x509.NewCertPool()
	if !rootCAs.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("append trace ca file: invalid pem data")
	}

	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    rootCAs,
	}, nil
}
