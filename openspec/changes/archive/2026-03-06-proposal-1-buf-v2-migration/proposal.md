## Why

当前项目使用 Buf v1 配置，proto 文件集中在 `api/protos/` 目录，所有生成配置通过 `buf.go.gen.yaml` 的 `go_package_prefix` 和 `override` 管理。随着项目演进，需要将 proto 文件分散到各服务目录，但 Buf v1 workspace 无法优雅地聚合多个分散的 proto 源。同时，`clean: true` 导致无法在 `api/gen/go/` 放置 `go.mod` 文件。迁移到 Buf v2 workspace 模式可以解决这些问题，为后续的服务独立化和 Go workspace 迁移奠定基础。

## What Changes

- 创建根目录 `buf.yaml` (v2 workspace)，聚合框架和服务的 proto 源
- 移动所有 `buf.*.gen.yaml` 配置文件从 `api/` 到根目录
- 删除旧的 `api/buf.work.yaml` (v1 workspace)
- 为所有 proto 文件添加 `option go_package` 声明，明确指定生成路径
- 简化 `buf.go.gen.yaml`，删除 `go_package_prefix` 和所有 `override` 配置
- 更新生成路径：`out: gen/go` → `out: api/gen/go`

## Capabilities

### New Capabilities
- `buf-v2-workspace`: Buf v2 workspace 配置，支持聚合多个 proto 源目录
- `proto-go-package-declaration`: 所有 proto 文件显式声明 go_package 选项

### Modified Capabilities
无

## Impact

- **Proto 文件**: 所有业务 proto 文件需要添加 `option go_package` 声明（auth, user, test, servora, sayhello, pagination 等）
- **Buf 配置**: 配置文件位置从 `api/` 移到根目录，影响开发者习惯和 CI/CD 脚本
- **生成代码**: import 路径保持不变（`github.com/horonlee/servora/api/gen/go/...`），现有代码无需修改
- **构建流程**: `make gen` 命令需要更新，但对开发者透明
- **兼容性**: 与现有代码完全兼容，不破坏任何 import 路径
