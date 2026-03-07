## Why

当前项目使用单一 `go.mod` 管理所有依赖，框架代码（`pkg/`, `cmd/svr/`）和业务服务（servora, sayhello）的依赖混在一起。这导致框架无法独立发布，服务无法独立版本管理。同时，`api/gen/go/` 目录因为 `buf generate` 的 `clean: true` 无法放置 `go.mod`，导致生成代码无法作为独立模块。采用 Go workspace 模式可以将项目拆分为多个独立模块，同时保持本地开发的便利性，为后续的服务独立化和 Git Submodule 拆分奠定基础。

## What Changes

- 创建 `api/gen/go.mod`（module `github.com/horonlee/servora/api/gen`）
- 创建 `app/servora/service/go.mod`（module `github.com/horonlee/servora/app/servora/service`）
- 创建 `app/sayhello/service/go.mod`（module `github.com/horonlee/servora/app/sayhello/service`）
- 精简根 `go.mod`，移除服务特定依赖（ent, gorm, 数据库驱动等），只保留框架依赖
- 创建 `go.work` 聚合所有模块（根模块、api/gen、servora、sayhello）
- 更新 `.gitignore`，移除 `go.work` 忽略规则（workspace 配置需要提交）

## Capabilities

### New Capabilities
- `go-workspace-config`: Go workspace 配置，支持多模块本地开发
- `api-gen-module`: 独立的 API 生成代码模块
- `service-modules`: 服务独立模块（servora, sayhello）

### Modified Capabilities
无

## Impact

- **依赖管理**: 从单一 `go.mod` 变为 4 个独立模块，每个模块独立管理依赖
- **构建流程**: 本地开发通过 `go.work` 自动解析依赖，无需手动 `replace`
- **Import 路径**: 保持不变，现有代码无需修改
- **CI/CD**: 需要在 CI 中先执行 `make gen` 生成 `api/gen/go/` 代码
- **服务独立性**: 为未来的 Git Submodule 拆分和独立 CI/CD 奠定基础
- **兼容性**: 完全向后兼容，不破坏任何现有功能
