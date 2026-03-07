## 为什么

当前 servora 框架中，每个服务的 `NewHTTPServer` 和 `NewGRPCServer` 函数包含大量重复的样板代码（~50 行/服务），包括：Network/Addr/Timeout 配置、TLS 证书加载、Logger 创建等。这导致：
1. 新服务创建成本高，需要复制粘贴大量代码
2. 配置逻辑分散，难以统一维护
3. 框架升级时需要逐个服务修改

## 变更内容

- **新增** `pkg/transport/server/http.go`：提供 `NewHTTPServer` 工厂函数和 Functional Options API
- **新增** `pkg/transport/server/grpc.go`：提供 `NewGRPCServer` 工厂函数和 Functional Options API
- **新增** `pkg/transport/server/tls.go`：TLS 证书加载的内部 helper
- **新增** `pkg/transport/server/plugin.go`：可插拔协议扩展接口（仅定义，预留 WebSocket/MCP/GraphQL）
- **修改** `app/servora/service/internal/server/http.go`：迁移使用新工厂
- **修改** `app/servora/service/internal/server/grpc.go`：迁移使用新工厂
- **修改** `app/sayhello/service/internal/server/grpc.go`：迁移使用新工厂

## 功能 (Capabilities)

### 新增功能

- `http-server-factory`: HTTP Server 工厂，支持 Functional Options 配置（Config/Logger/Middleware/CORS/Metrics）和 Registrar 模式的服务注册
- `grpc-server-factory`: gRPC Server 工厂，支持 Functional Options 配置（Config/Logger/Middleware）和 Registrar 模式的服务注册
- `server-plugin-interface`: 可插拔协议扩展接口定义，预留 WebSocket/MCP/GraphQL 等协议的扩展点

### 修改功能

- `middleware-chain`: 与新工厂集成，ChainBuilder 产出的中间件链通过 `WithHTTPMiddleware`/`WithGRPCMiddleware` 注入

## 影响

- **代码**：`pkg/transport/server/` 新增 4 个文件，`app/*/service/internal/server/` 简化
- **API**：新增公开 API（`NewHTTPServer`、`NewGRPCServer`、各种 Option 函数）
- **依赖**：`pkg/transport/server` 将依赖 `pkg/middleware/cors` 和 `pkg/governance/telemetry`
- **代码量**：服务端 server 代码从 ~55 行降至 ~15 行
