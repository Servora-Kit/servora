## 上下文

当前 servora 框架中，HTTP 和 gRPC 服务器的中间件链在各服务的 `internal/server/` 中独立定义。以 servora 服务为例：

```go
// app/servora/service/internal/server/http.go
var ms []middleware.Middleware
ms = append(ms, recovery.Recovery())
if trace != nil && trace.Endpoint != "" {
    ms = append(ms, tracing.Server())
}
ms = append(ms,
    logging.Server(httpLogger),
    ratelimit.Server(),
    validate.ProtoValidate(),
)
if m != nil {
    ms = append(ms, metrics.Server(...))
}
```

sayhello 服务有类似但略有不同的实现（缺少 ratelimit）。这种模式导致代码重复和潜在的不一致性。

Kratos 框架使用统一的 `middleware.Middleware` 类型，HTTP 和 gRPC 共享同一套中间件定义，这为抽取提供了基础。

## 目标 / 非目标

**目标：**
- 提供统一的中间件链构建器，消除服务间的代码重复
- 保证中间件顺序的正确性（recovery 必须第一，metrics 在认证之前）
- 支持可选配置（tracing、metrics、ratelimit）
- 保持与现有 Wire 依赖注入的兼容性

**非目标：**
- 不抽取业务特定的中间件（如 auth/selector）
- 不改变 CORS 的处理方式（它是 HTTP Filter，不是 Middleware）
- 不提供运行时动态修改中间件链的能力

## 决策

### 1. 使用 Builder 模式而非简单函数

**选择**：Builder 模式
**替代方案**：简单的 `NewDefaultChain(cfg ChainConfig)` 函数

**理由**：
- Builder 模式提供更好的可读性和链式调用体验
- 可以清晰地表达可选配置（`WithTrace`、`WithMetrics`、`WithoutRateLimit`）
- 未来扩展更灵活（可以添加 `WithCustom` 等方法）

### 2. 中间件顺序固定

**选择**：顺序由 Builder 内部保证，用户无法改变
**替代方案**：允许用户自定义顺序

**理由**：
- 中间件顺序有最佳实践（recovery 必须第一，tracing 在 logging 之前）
- 错误的顺序可能导致难以调试的问题
- 如果用户需要完全自定义，可以不使用 Builder

**固定顺序**：
```
1. Recovery  - 捕获 panic，防止服务崩溃
2. Tracing   - 分布式链路追踪（可选）
3. Logging   - 请求/响应日志
4. RateLimit - 限流保护（默认启用，可禁用）
5. Validate  - Proto 参数校验
6. Metrics   - 指标收集（可选）
```

### 3. RateLimit 默认启用

**选择**：默认启用，提供 `WithoutRateLimit()` 禁用
**替代方案**：默认禁用，提供 `WithRateLimit()` 启用

**理由**：
- 限流是生产环境的基本保护，默认启用更安全
- 本地开发如需禁用，可以调用 `WithoutRateLimit()` 或直接注释 pkg 代码

### 4. 文件位置

**选择**：`pkg/transport/server/middleware/chain.go`
**替代方案**：`pkg/middleware/chain.go`

**理由**：
- 这是 server 层的中间件链，放在 `transport/server/` 下更符合语义
- `pkg/middleware/` 已有 `cors/` 和 `whitelist.go`，它们是具体的中间件实现
- 新的 `chain.go` 是中间件的组合器，与具体实现分开更清晰

### 5. HTTP 和 gRPC 不分开

**选择**：统一的 `ChainBuilder`，通过 Logger 区分
**替代方案**：分别提供 `HTTPChainBuilder` 和 `GRPCChainBuilder`

**理由**：
- Kratos 的 `middleware.Middleware` 类型是统一的
- 唯一的差异是 Logger 的 module 名，这由调用方传入
- 分开会导致代码重复

## 风险 / 权衡

| 风险 | 缓解措施 |
|------|----------|
| 顺序固定可能不满足特殊需求 | 用户可以不使用 Builder，手动构建切片 |
| 新增中间件需要修改 Builder | 提供详细注释，说明如何扩展 |
| 服务迁移可能引入 bug | 迁移后运行完整测试，对比中间件链输出 |

## API 设计

```go
// pkg/transport/server/middleware/chain.go

// ChainBuilder 构建标准中间件链。
//
// 中间件顺序（按 Build 输出顺序）：
//   1. Recovery  - 捕获 panic，防止服务崩溃
//   2. Tracing   - 分布式链路追踪（可选，需调用 WithTrace）
//   3. Logging   - 请求/响应日志
//   4. RateLimit - 限流保护（默认启用，可调用 WithoutRateLimit 禁用）
//   5. Validate  - Proto 参数校验
//   6. Metrics   - 指标收集（可选，需调用 WithMetrics）
//
// 使用示例：
//
//   httpLogger := logger.With(l, logger.WithModule("http/server/my-service"))
//   ms := middleware.NewChainBuilder(httpLogger).
//       WithTrace(trace).
//       WithMetrics(mtc).
//       Build()
//   ms = append(ms, authMiddleware...)
type ChainBuilder struct {
    logger      log.Logger
    trace       *conf.Trace
    metrics     *telemetry.Metrics
    rateLimit   bool // 默认 true
}

// NewChainBuilder 创建中间件链构建器。
// logger 参数是必须的，用于 logging 中间件。
func NewChainBuilder(l log.Logger) *ChainBuilder

// WithTrace 启用分布式链路追踪。
// 如果 t 为 nil 或 t.Endpoint 为空，则跳过 tracing 中间件。
func (b *ChainBuilder) WithTrace(t *conf.Trace) *ChainBuilder

// WithMetrics 启用指标收集。
// 如果 m 为 nil，则跳过 metrics 中间件。
func (b *ChainBuilder) WithMetrics(m *telemetry.Metrics) *ChainBuilder

// WithoutRateLimit 禁用限流中间件。
// 默认情况下限流是启用的。
func (b *ChainBuilder) WithoutRateLimit() *ChainBuilder

// Build 构建并返回中间件切片。
// 返回的切片可以通过 append 追加业务特定的中间件。
func (b *ChainBuilder) Build() []middleware.Middleware
```
