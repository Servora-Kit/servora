## 上下文

当前 servora 框架的 Server 创建采用 Kratos 官方推荐的 "Provider 模式"，每个服务在 `internal/server/` 下手写 `NewHTTPServer` 和 `NewGRPCServer` 函数。这种模式虽然显式、DI 友好，但存在大量重复代码：

- servora HTTP: ~55 行，其中 ~35 行是样板代码
- servora gRPC: ~40 行，其中 ~25 行是样板代码
- sayhello gRPC: ~56 行，其中 ~40 行是样板代码

样板代码包括：Network/Addr/Timeout 配置、TLS 证书加载、Logger 设置等。

## 目标 / 非目标

**目标：**
- 将服务端 Server 创建代码从 ~55 行降至 ~15 行
- 提供符合 Go 惯用法的 Functional Options API
- 支持 Registrar 模式解耦服务注册
- 将 CORS 和 Metrics 作为可选 Option 集成
- 预留可插拔协议扩展点（WebSocket/MCP/GraphQL）

**非目标：**
- 不实现具体的 Plugin（WebSocket/MCP/GraphQL），仅定义接口
- 不改变 Wire 依赖注入的整体模式
- 不修改 `pkg/transport/server/middleware/` 的 ChainBuilder 实现
- 不支持 SSE Server 的统一创建（当前用不上）

## 决策

### 决策 1: Functional Options vs Config Struct

**选择**: Functional Options 模式

**理由**:
- 符合 Go 社区惯用法（gRPC-Go、Kratos 本身都采用此模式）
- 支持可选参数，零值安全
- 向后兼容性好，新增 Option 不破坏现有调用
- 链式调用可读性好

**替代方案**:
- Config Struct：需要传递完整结构体，可选字段处理麻烦
- Builder 模式：需要维护 Builder 状态，对于简单场景过度设计

### 决策 2: Registrar 模式 vs 参数传递

**选择**: Registrar 闭包模式

```go
type HTTPRegistrar func(*http.Server)

func WithHTTPServices(registrars ...HTTPRegistrar) HTTPServerOption
```

**理由**:
- 函数签名固定，新增服务不需要修改工厂函数
- 服务注册逻辑外置，工厂函数保持通用
- 与 Wire 配合良好，闭包捕获服务实例

**替代方案**:
- 参数传递：每加一个服务要改函数签名，Wire 依赖图变复杂
- 接口注册：需要定义额外的 Registrar 接口，增加复杂度

### 决策 3: TLS 错误处理

**选择**: panic（严重错误直接退出）

**理由**:
- TLS 配置错误是启动时的致命错误，服务无法正常运行
- 与当前服务内 `log.Fatal` 行为一致
- 简化 API，不需要返回 error

**替代方案**:
- 返回 error：调用方需要处理错误，但 TLS 失败后服务也无法运行

### 决策 4: CORS/Metrics 集成方式

**选择**: 方案 C - Option 接受高层对象

```go
func WithCORS(c *conf.CORS) HTTPServerOption
func WithMetrics(m *telemetry.Metrics) HTTPServerOption
```

**理由**:
- 使用更简洁，不需要 `.Handler`
- 实现保持在原位置（`pkg/middleware/cors`、`pkg/governance/telemetry`）
- 职责清晰：原包负责实现，server 包负责组合

**替代方案**:
- 传 Handler：需要知道内部结构（如 `mtc.Handler`）
- 移动到 server 包：CORS 可能用于非 server 场景，移动不合理

### 决策 5: Plugin 扩展设计

**选择**: 仅定义接口，不实现具体 Plugin

```go
type PluginKind string
const (
    PluginWebSocket PluginKind = "websocket"
    PluginMCP       PluginKind = "mcp"
    PluginGraphQL   PluginKind = "graphql"
)

type ServerPlugin interface {
    Server
    Kind() PluginKind
}
```

**理由**:
- 预留扩展点，不增加当前实现复杂度
- 遵循 YAGNI 原则，需要时再实现
- 接口定义清晰，未来实现有明确契约

## 风险 / 权衡

### 风险 1: 依赖增加
**风险**: `pkg/transport/server` 将依赖 `pkg/middleware/cors` 和 `pkg/governance/telemetry`

**缓解**: 这些都是框架内部包，依赖关系合理。如果未来需要解耦，可以通过接口抽象。

### 风险 2: 迁移成本
**风险**: 现有服务需要修改 `internal/server/` 代码

**缓解**: 
- 迁移是简化代码，不是增加复杂度
- 可以逐个服务迁移，不需要一次性完成
- 旧模式仍然可用，不是破坏性变更

### 风险 3: Option 顺序敏感
**风险**: 某些 Option 可能对顺序敏感（如 Middleware）

**缓解**: 
- 在文档中明确说明 Option 的应用顺序
- Middleware 按传入顺序组装，与 Kratos 原生行为一致

## 目录结构

```
pkg/transport/server/
├── server.go          # 已有：Lifecycle, Server, EndpointProvider 接口
├── middleware/
│   ├── chain.go       # 已有：ChainBuilder
│   └── chain_test.go  # 已有：测试
├── http.go            # 新增：HTTPServerOption, NewHTTPServer
├── grpc.go            # 新增：GRPCServerOption, NewGRPCServer
├── tls.go             # 新增：mustLoadTLS (内部 helper)
├── plugin.go          # 新增：PluginKind, ServerPlugin 接口
└── sse/               # 已有：SSE 实现 (保持不变)
```
