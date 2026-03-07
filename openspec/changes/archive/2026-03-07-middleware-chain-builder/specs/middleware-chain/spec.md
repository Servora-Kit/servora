## 新增需求

### 需求:ChainBuilder 必须提供 Builder 模式构建中间件链

系统必须提供 `ChainBuilder` 类型，支持链式调用配置中间件，最终通过 `Build()` 方法返回 `[]middleware.Middleware` 切片。

#### 场景:基本构建

- **当** 调用 `NewChainBuilder(logger).Build()`
- **那么** 返回包含 recovery、logging、ratelimit、validate 四个中间件的切片

#### 场景:链式配置

- **当** 调用 `NewChainBuilder(logger).WithTrace(trace).WithMetrics(mtc).Build()`
- **那么** 返回包含 recovery、tracing、logging、ratelimit、validate、metrics 六个中间件的切片

### 需求:中间件顺序必须固定

系统必须保证 `Build()` 返回的中间件切片顺序为：recovery → tracing → logging → ratelimit → validate → metrics。用户禁止通过 Builder API 改变此顺序。

#### 场景:顺序验证

- **当** 调用 `NewChainBuilder(logger).WithTrace(trace).WithMetrics(mtc).Build()`
- **那么** 返回的切片中 recovery 在索引 0，tracing 在索引 1，logging 在索引 2，ratelimit 在索引 3，validate 在索引 4，metrics 在索引 5

### 需求:Tracing 中间件必须可选

系统必须支持通过 `WithTrace(*conf.Trace)` 方法启用 tracing 中间件。如果未调用此方法或传入 nil 或 `Endpoint` 为空，则禁止添加 tracing 中间件。

#### 场景:启用 tracing

- **当** 调用 `WithTrace(&conf.Trace{Endpoint: "http://jaeger:14268"})` 且 Endpoint 非空
- **那么** Build 返回的切片包含 tracing 中间件

#### 场景:跳过 tracing（未调用）

- **当** 未调用 `WithTrace` 方法
- **那么** Build 返回的切片不包含 tracing 中间件

#### 场景:跳过 tracing（Endpoint 为空）

- **当** 调用 `WithTrace(&conf.Trace{Endpoint: ""})` 且 Endpoint 为空
- **那么** Build 返回的切片不包含 tracing 中间件

### 需求:Metrics 中间件必须可选

系统必须支持通过 `WithMetrics(*telemetry.Metrics)` 方法启用 metrics 中间件。如果未调用此方法或传入 nil，则禁止添加 metrics 中间件。

#### 场景:启用 metrics

- **当** 调用 `WithMetrics(mtc)` 且 mtc 非 nil
- **那么** Build 返回的切片包含 metrics 中间件

#### 场景:跳过 metrics

- **当** 未调用 `WithMetrics` 方法或传入 nil
- **那么** Build 返回的切片不包含 metrics 中间件

### 需求:RateLimit 中间件必须默认启用

系统必须默认启用 ratelimit 中间件。用户可以通过 `WithoutRateLimit()` 方法禁用。

#### 场景:默认启用

- **当** 调用 `NewChainBuilder(logger).Build()` 且未调用 `WithoutRateLimit()`
- **那么** Build 返回的切片包含 ratelimit 中间件

#### 场景:显式禁用

- **当** 调用 `NewChainBuilder(logger).WithoutRateLimit().Build()`
- **那么** Build 返回的切片不包含 ratelimit 中间件

### 需求:Logger 参数必须必填

系统必须要求 `NewChainBuilder` 接收一个非 nil 的 `log.Logger` 参数。此 Logger 用于 logging 中间件。

#### 场景:正常创建

- **当** 调用 `NewChainBuilder(validLogger)` 且 logger 非 nil
- **那么** 成功创建 ChainBuilder 实例

### 需求:Build 返回的切片必须可追加

系统必须保证 `Build()` 返回的 `[]middleware.Middleware` 切片可以通过 `append` 追加业务特定的中间件（如 auth、selector）。

#### 场景:追加业务中间件

- **当** 调用 `ms := builder.Build()` 后执行 `ms = append(ms, authMiddleware)`
- **那么** ms 切片包含基础中间件和追加的 authMiddleware
