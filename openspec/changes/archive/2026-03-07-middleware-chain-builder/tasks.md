## 1. 核心实现

- [x] 1.1 创建 `pkg/transport/server/middleware/` 目录
- [x] 1.2 实现 `chain.go`：ChainBuilder 结构体和 NewChainBuilder 函数
- [x] 1.3 实现 WithTrace、WithMetrics、WithoutRateLimit 方法
- [x] 1.4 实现 Build 方法，按固定顺序组装中间件切片
- [x] 1.5 添加详细的 GoDoc 注释，说明中间件顺序和使用示例

## 2. 单元测试

- [x] 2.1 创建 `chain_test.go`：测试基本构建（仅必填中间件）
- [x] 2.2 测试 WithTrace 启用/跳过逻辑
- [x] 2.3 测试 WithMetrics 启用/跳过逻辑
- [x] 2.4 测试 WithoutRateLimit 禁用逻辑
- [x] 2.5 测试中间件顺序正确性

## 3. 服务迁移

- [x] 3.1 重构 `app/servora/service/internal/server/http.go` 使用 ChainBuilder
- [x] 3.2 重构 `app/servora/service/internal/server/grpc.go` 使用 ChainBuilder
- [x] 3.3 重构 `app/sayhello/service/internal/server/grpc.go` 使用 ChainBuilder
- [x] 3.4 更新各服务的 import 路径

## 4. 验证

- [x] 4.1 运行 `go build ./...` 确保编译通过
- [x] 4.2 运行 `go test ./pkg/transport/server/middleware/...` 确保测试通过
- [x] 4.3 运行 `make compose.dev` 启动服务，验证中间件链正常工作
- [x] 4.4 运行 `make lint.go` 确保代码风格符合规范
