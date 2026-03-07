## 为什么

当前 servora 框架中，每个微服务（servora、sayhello）都在各自的 `internal/server/` 中重复定义基础中间件链（recovery、tracing、logging、ratelimit、validate、metrics）。这导致：
1. 代码重复约 15-20 行/服务
2. 中间件顺序可能不一致
3. 新服务需要复制粘贴，容易遗漏或出错

现在做是因为框架正在完善基础设施层，需要在服务数量增加前建立统一的中间件组合模式。

## 变更内容

- **新增**：在 `pkg/transport/server/middleware/` 提供 `ChainBuilder`，使用 Builder 模式构建标准中间件链
- **新增**：支持可选的 tracing、metrics 配置，以及可禁用的 ratelimit
- **修改**：重构 `app/servora/service/internal/server/http.go` 和 `grpc.go` 使用新的 ChainBuilder
- **修改**：重构 `app/sayhello/service/internal/server/grpc.go` 使用新的 ChainBuilder

## 功能 (Capabilities)

### 新增功能

- `middleware-chain`: 提供统一的中间件链构建器，支持 Builder 模式配置 recovery、tracing、logging、ratelimit、validate、metrics 的组合

### 修改功能

## 影响

- **代码**：
  - 新增 `pkg/transport/server/middleware/chain.go`
  - 修改 `app/servora/service/internal/server/http.go`
  - 修改 `app/servora/service/internal/server/grpc.go`
  - 修改 `app/sayhello/service/internal/server/grpc.go`
- **API**：无破坏性变更，服务侧中间件构造方式变更但对外接口不变
- **依赖**：无新增依赖，复用现有 Kratos middleware 包
