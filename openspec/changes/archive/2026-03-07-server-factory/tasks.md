## 1. 核心工厂实现

- [x] 1.1 创建 `pkg/transport/server/tls.go`，实现 `mustLoadTLS` 内部 helper 函数
- [x] 1.2 创建 `pkg/transport/server/http.go`，定义 `HTTPRegistrar` 类型和 `httpServerOptions` 结构体
- [x] 1.3 实现 HTTP Server Options：`WithHTTPConfig`、`WithHTTPLogger`、`WithHTTPMiddleware`
- [x] 1.4 实现 HTTP Server Options：`WithCORS`、`WithMetrics`、`WithHTTPServices`
- [x] 1.5 实现 `NewHTTPServer` 工厂函数
- [x] 1.6 创建 `pkg/transport/server/grpc.go`，定义 `GRPCRegistrar` 类型和 `grpcServerOptions` 结构体
- [x] 1.7 实现 gRPC Server Options：`WithGRPCConfig`、`WithGRPCLogger`、`WithGRPCMiddleware`、`WithGRPCServices`
- [x] 1.8 实现 `NewGRPCServer` 工厂函数

## 2. Plugin 接口定义

- [x] 2.1 创建 `pkg/transport/server/plugin.go`，定义 `PluginKind` 类型和常量（WebSocket/MCP/GraphQL）
- [x] 2.2 定义 `ServerPlugin` 接口，继承 `Server` 接口并添加 `Kind()` 方法

## 3. 单元测试

- [x] 3.1 创建 `pkg/transport/server/http_test.go`，测试 HTTP Server 工厂的各种 Option 组合
- [x] 3.2 创建 `pkg/transport/server/grpc_test.go`，测试 gRPC Server 工厂的各种 Option 组合

## 4. 服务迁移

- [x] 4.1 迁移 `app/servora/service/internal/server/http.go` 使用新工厂
- [x] 4.2 迁移 `app/servora/service/internal/server/grpc.go` 使用新工厂
- [x] 4.3 迁移 `app/sayhello/service/internal/server/grpc.go` 使用新工厂
- [x] 4.4 更新 servora 服务的 Wire ProviderSet（如有需要）
- [x] 4.5 更新 sayhello 服务的 Wire ProviderSet（如有需要）

## 5. 验证

- [x] 5.1 执行 `go build ./...` 确保编译通过
- [x] 5.2 执行 `go test ./pkg/transport/server/...` 确保测试通过
- [x] 5.3 执行 `make lint.go` 确保代码风格合规
- [x] 5.4 执行 `make wire` 重新生成 Wire 代码
- [x] 5.5 执行 `make compose.dev` 验证服务正常启动（需用户手动验证）
