## Why

当前所有 proto 文件集中在 `api/protos/` 目录，框架公共 proto（conf, pagination）和业务 proto（auth, user, servora, sayhello）混在一起。这导致业务 API 变更需要在框架仓库操作，服务无法独立管理自己的 API 定义。在完成 Buf v2 迁移和 Go 模块拆分后，需要将业务 proto 移到各服务目录，实现 proto 定义跟随服务，同时更新构建系统以适应新的目录结构。

## What Changes

- **BREAKING** 移动业务 proto 文件：
  - `api/protos/auth/` → `app/servora/service/api/protos/auth/`
  - `api/protos/user/` → `app/servora/service/api/protos/user/`
  - `api/protos/test/` → `app/servora/service/api/protos/test/`
  - `api/protos/servora/` → `app/servora/service/api/protos/servora/`
  - `api/protos/sayhello/` → `app/sayhello/service/api/protos/sayhello/`
- 框架保留公共 proto：`conf/`, `pagination/`, `template/`, `k8s/`
- 更新根 `buf.yaml` 的 `modules` 路径，聚合服务 proto
- 更新根 `Makefile` 和 `app.mk`，适配新的 buf 配置位置
- 更新 `.gitignore`，确保 `api/gen/go.mod` 不被忽略

## Capabilities

### New Capabilities
- `service-proto-organization`: 服务 proto 目录组织结构
- `build-system-update`: 更新后的构建系统配置

### Modified Capabilities
无

## Impact

- **Proto 文件路径**: 业务 proto 从 `api/protos/` 移到 `app/*/service/api/protos/`，影响 IDE 跳转和文档引用
- **Buf 配置**: 根 `buf.yaml` 需要聚合多个 proto 源目录
- **生成代码**: import 路径保持不变（`github.com/horonlee/servora/api/gen/go/...`），现有代码无需修改
- **构建流程**: `make gen` 命令更新，但对开发者透明
- **Proto 跨引用**: servora 引用 auth/user/test 的跨引用通过 Buf v2 workspace 自动解析
- **服务独立性**: 为未来的 Git Submodule 拆分奠定基础，服务可以独立管理自己的 API 定义
