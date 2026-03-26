// Package middleware 提供服务器中间件链构建工具。
package middleware

import (
	"github.com/go-kratos/kratos/contrib/middleware/validate/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/metrics"
	"github.com/go-kratos/kratos/v2/middleware/ratelimit"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"

	"github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/obs/telemetry"
)

// ChainBuilder 构建标准中间件链。
//
// 中间件顺序（按 Build 输出顺序）：
//
//  1. Recovery  - 捕获 panic，防止服务崩溃
//  2. Tracing   - 分布式链路追踪（可选，需调用 WithTrace）
//  3. Logging   - 请求/响应日志
//  4. RateLimit - 限流保护（默认启用，可调用 WithoutRateLimit 禁用）
//  5. Validate  - Proto 参数校验
//  6. Metrics   - 指标收集（可选，需调用 WithMetrics）
//
// 使用示例：
//
//	httpLogger := logger.With(l, "http/server/my-service")
//	ms := middleware.NewChainBuilder(httpLogger).
//	    WithTrace(trace).
//	    WithMetrics(mtc).
//	    Build()
//	ms = append(ms, authMiddleware...)
//
// 注意：
//   - HTTP 和 gRPC 共享同一个 ChainBuilder，通过传入不同的 Logger 区分
//   - 返回的切片可以通过 append 追加业务特定的中间件（如 auth、selector）
//   - 如果需要完全自定义中间件顺序，请不要使用此 Builder，手动构建切片
type ChainBuilder struct {
	logger    log.Logger
	trace     *conf.Trace
	metrics   *telemetry.Metrics
	rateLimit bool // 默认 true
}

// NewChainBuilder 创建中间件链构建器。
//
// logger 参数是必须的，用于 logging 中间件。
// 建议使用 logger.With(l, "http/server/xxx") 或
// logger.With(l, "grpc/server/xxx") 来区分协议。
func NewChainBuilder(l log.Logger) *ChainBuilder {
	return &ChainBuilder{
		logger:    l,
		rateLimit: true, // 默认启用限流
	}
}

// WithTrace 启用分布式链路追踪。
//
// 如果 t 为 nil 或 t.Endpoint 为空，则跳过 tracing 中间件。
// 这允许在未配置 trace endpoint 的环境中优雅降级。
func (b *ChainBuilder) WithTrace(t *conf.Trace) *ChainBuilder {
	b.trace = t
	return b
}

// WithMetrics 启用指标收集。
//
// 如果 m 为 nil，则跳过 metrics 中间件。
// metrics 中间件会记录请求计数和延迟分布。
func (b *ChainBuilder) WithMetrics(m *telemetry.Metrics) *ChainBuilder {
	b.metrics = m
	return b
}

// WithoutRateLimit 禁用限流中间件。
//
// 默认情况下限流是启用的，这是生产环境的推荐配置。
// 仅在以下场景考虑禁用：
//   - 本地开发/测试环境
//   - 内部服务间调用（已有上游限流）
//   - 性能压测
func (b *ChainBuilder) WithoutRateLimit() *ChainBuilder {
	b.rateLimit = false
	return b
}

// Build 构建并返回中间件切片。
//
// 中间件按以下固定顺序添加：
//  1. Recovery  - 必须第一个，捕获所有后续中间件的 panic
//  2. Tracing   - 在 logging 之前，确保日志可以关联 trace ID
//  3. Logging   - 记录请求/响应
//  4. RateLimit - 在业务逻辑之前进行限流
//  5. Validate  - Proto 参数校验
//  6. Metrics   - 记录请求指标
//
// 返回的切片可以通过 append 追加业务特定的中间件。
func (b *ChainBuilder) Build() []middleware.Middleware {
	var ms []middleware.Middleware

	// 1. Recovery - 必须第一个
	ms = append(ms, recovery.Recovery())

	// 2. Tracing - 可选
	if b.trace != nil && b.trace.Endpoint != "" {
		ms = append(ms, tracing.Server())
	}

	// 3. Logging - 必须
	ms = append(ms, logging.Server(b.logger))

	// 4. RateLimit - 默认启用
	if b.rateLimit {
		ms = append(ms, ratelimit.Server())
	}

	// 5. Validate - 必须
	ms = append(ms, validate.ProtoValidate())

	// 6. Metrics - 可选
	if b.metrics != nil {
		ms = append(ms, metrics.Server(
			metrics.WithSeconds(b.metrics.Seconds),
			metrics.WithRequests(b.metrics.Requests),
		))
	}

	return ms
}
