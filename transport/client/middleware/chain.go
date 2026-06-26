package middleware

import (
	"log/slog"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
	"github.com/Servora-Kit/servora/obs/metrics"
	kmetrics "github.com/go-kratos/kratos/contrib/otel/v3/metrics"
	"github.com/go-kratos/kratos/contrib/otel/v3/tracing"
	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/middleware/circuitbreaker"
	"github.com/go-kratos/kratos/v3/middleware/logging"
	"github.com/go-kratos/kratos/v3/middleware/recovery"
)

// ChainBuilder 构建标准 client 中间件链。
//
// 默认链不包含 jwt token 透传：跨服务调用时若需将入站 Bearer token
// 转发到下游，调用方需显式 append `security/authn/jwt.Client()`，详见该
// sub-package godoc 与 design.md Decision 5。
type ChainBuilder struct {
	logger  *slog.Logger
	trace   *corev1.Trace
	metrics *metrics.Metrics
	circuit bool
}

// NewChainBuilder 创建 client 中间件链构建器。
func NewChainBuilder(l *slog.Logger) *ChainBuilder {
	return &ChainBuilder{
		logger:  l,
		circuit: true,
	}
}

// WithTrace 启用 client tracing 中间件。
func (b *ChainBuilder) WithTrace(t *corev1.Trace) *ChainBuilder {
	b.trace = t
	return b
}

// WithMetrics 启用 client metrics 中间件。
func (b *ChainBuilder) WithMetrics(m *metrics.Metrics) *ChainBuilder {
	b.metrics = m
	return b
}

// WithoutCircuitBreaker 禁用 client 熔断中间件。
func (b *ChainBuilder) WithoutCircuitBreaker() *ChainBuilder {
	b.circuit = false
	return b
}

// Build 构建并返回 client 中间件切片。
func (b *ChainBuilder) Build() []middleware.Middleware {
	ms := make([]middleware.Middleware, 0, 6)
	ms = append(ms, recovery.Recovery())

	if b.trace != nil && b.trace.Endpoint != "" {
		ms = append(ms, tracing.Client())
	}

	ms = append(ms, logging.Client(b.logger))

	if b.circuit {
		ms = append(ms, circuitbreaker.Client())
	}

	if b.metrics != nil {
		ms = append(ms, kmetrics.Client(
			kmetrics.WithSeconds(b.metrics.ClientSeconds),
			kmetrics.WithRequests(b.metrics.ClientRequests),
		))
	}

	return ms
}
