## 修改需求

### 需求:ChainBuilder 与工厂集成

ChainBuilder 产出的中间件链必须能够通过 `WithHTTPMiddleware` 和 `WithGRPCMiddleware` Option 注入到新的 Server 工厂中。

#### 场景:HTTP Server 使用 ChainBuilder

- **当** 使用 `coremw.NewChainBuilder(l).WithTrace(trace).WithMetrics(mtc).Build()` 构建中间件链
- **那么** 必须能够通过 `server.WithHTTPMiddleware(mw...)` 传入 `NewHTTPServer`

#### 场景:gRPC Server 使用 ChainBuilder

- **当** 使用 `coremw.NewChainBuilder(l).WithTrace(trace).WithMetrics(mtc).WithoutRateLimit().Build()` 构建中间件链
- **那么** 必须能够通过 `server.WithGRPCMiddleware(mw...)` 传入 `NewGRPCServer`

#### 场景:中间件类型兼容

- **当** ChainBuilder 返回 `[]middleware.Middleware`
- **那么** 必须与 `WithHTTPMiddleware` 和 `WithGRPCMiddleware` 的参数类型兼容（均为 `...middleware.Middleware`）
