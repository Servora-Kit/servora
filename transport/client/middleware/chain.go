package middleware

import (
	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/obs/telemetry"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/circuitbreaker"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/metrics"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
)

// ChainBuilder 构建标准 client 中间件链。
type ChainBuilder struct {
	logger  log.Logger
	trace   *conf.Trace
	metrics *telemetry.Metrics
	circuit bool
}

// NewChainBuilder 创建 client 中间件链构建器。
func NewChainBuilder(l log.Logger) *ChainBuilder {
	return &ChainBuilder{
		logger:  l,
		circuit: true,
	}
}

// WithTrace 启用 client tracing 中间件。
func (b *ChainBuilder) WithTrace(t *conf.Trace) *ChainBuilder {
	b.trace = t
	return b
}

// WithMetrics 启用 client metrics 中间件。
func (b *ChainBuilder) WithMetrics(m *telemetry.Metrics) *ChainBuilder {
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

	ms = append(ms, TokenPropagation())

	if b.metrics != nil {
		ms = append(ms, metrics.Client(
			metrics.WithSeconds(b.metrics.Seconds),
			metrics.WithRequests(b.metrics.Requests),
		))
	}

	return ms
}
