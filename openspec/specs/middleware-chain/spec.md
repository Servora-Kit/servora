## Purpose
定义 middleware-chain 的功能需求和验证场景。

## Requirements

### Requirement: ChainBuilder 必须提供 Builder 模式构建中间件链

系统必须提供 `ChainBuilder` 类型，支持链式调用配置中间件，最终通过 `Build()` 方法返回 `[]middleware.Middleware` 切片。

#### Scenario: 基本构建

- **WHEN** 调用 `NewChainBuilder(logger).Build()`
- **THEN** 返回包含 recovery、logging、ratelimit、validate 四个中间件的切片

#### Scenario: 链式配置

- **WHEN** 调用 `NewChainBuilder(logger).WithTrace(trace).WithMetrics(mtc).Build()`
- **THEN** 返回包含 recovery、tracing、logging、ratelimit、validate、metrics 六个中间件的切片

### Requirement: 中间件顺序必须固定

系统必须保证 `Build()` 返回的中间件切片顺序为：recovery → tracing → logging → ratelimit → validate → metrics。用户禁止通过 Builder API 改变此顺序。

#### Scenario: 顺序验证

- **WHEN** 调用 `NewChainBuilder(logger).WithTrace(trace).WithMetrics(mtc).Build()`
- **THEN** 返回的切片中 recovery 在索引 0，tracing 在索引 1，logging 在索引 2，ratelimit 在索引 3，validate 在索引 4，metrics 在索引 5

### Requirement: Tracing 中间件必须可选

系统必须支持通过 `WithTrace(*conf.Trace)` 方法启用 tracing 中间件。如果未调用此方法或传入 nil 或 `Endpoint` 为空，则禁止添加 tracing 中间件。

#### Scenario: 启用 tracing

- **WHEN** 调用 `WithTrace(&conf.Trace{Endpoint: "http://jaeger:14268"})` 且 Endpoint 非空
- **THEN** Build 返回的切片包含 tracing 中间件

#### Scenario: 跳过 tracing（未调用）

- **WHEN** 未调用 `WithTrace` 方法
- **THEN** Build 返回的切片不包含 tracing 中间件

#### Scenario: 跳过 tracing（Endpoint 为空）

- **WHEN** 调用 `WithTrace(&conf.Trace{Endpoint: ""})` 且 Endpoint 为空
- **THEN** Build 返回的切片不包含 tracing 中间件

### Requirement: Metrics 中间件必须可选

系统必须支持通过 `WithMetrics(*telemetry.Metrics)` 方法启用 metrics 中间件。如果未调用此方法或传入 nil，则禁止添加 metrics 中间件。

#### Scenario: 启用 metrics

- **WHEN** 调用 `WithMetrics(mtc)` 且 mtc 非 nil
- **THEN** Build 返回的切片包含 metrics 中间件

#### Scenario: 跳过 metrics

- **WHEN** 未调用 `WithMetrics` 方法或传入 nil
- **THEN** Build 返回的切片不包含 metrics 中间件

### Requirement: RateLimit 中间件必须默认启用

系统必须默认启用 ratelimit 中间件。用户可以通过 `WithoutRateLimit()` 方法禁用。

#### Scenario: 默认启用

- **WHEN** 调用 `NewChainBuilder(logger).Build()` 且未调用 `WithoutRateLimit()`
- **THEN** Build 返回的切片包含 ratelimit 中间件

#### Scenario: 显式禁用

- **WHEN** 调用 `NewChainBuilder(logger).WithoutRateLimit().Build()`
- **THEN** Build 返回的切片不包含 ratelimit 中间件

### Requirement: Logger 参数必须必填

系统必须要求 `NewChainBuilder` 接收一个非 nil 的 `log.Logger` 参数。此 Logger 用于 logging 中间件。

#### Scenario: 正常创建

- **WHEN** 调用 `NewChainBuilder(validLogger)` 且 logger 非 nil
- **THEN** 成功创建 ChainBuilder 实例

### Requirement: Build 返回的切片必须可追加

系统必须保证 `Build()` 返回的 `[]middleware.Middleware` 切片可以通过 `append` 追加业务特定的中间件（如 auth、selector）。

#### Scenario: 追加业务中间件

- **WHEN** 调用 `ms := builder.Build()` 后执行 `ms = append(ms, authMiddleware)`
- **THEN** ms 切片包含基础中间件和追加的 authMiddleware
