# api-gen-module 规范

## Purpose
待定 - 由归档变更 proposal-2-go-modules-workspace 创建。归档后请更新目的。
## Requirements
### Requirement: 必须创建 api/gen/go.mod 模块

系统必须在 `api/gen/` 目录创建独立的 `go.mod` 文件，将生成的 proto 代码作为独立模块管理。

#### Scenario: go.mod 位置

- **WHEN** 创建 api/gen 模块
- **THEN** `go.mod` 必须位于 `api/gen/go.mod`（而不是 `api/gen/go/go.mod`）
- **THEN** 模块路径必须为 `module github.com/Servora-Kit/servora/api/gen`

#### Scenario: 避免被 buf generate 删除

- **WHEN** 执行 `buf generate` 并配置 `clean: true`
- **THEN** `api/gen/go.mod` 必须不被删除
- **THEN** 只有 `api/gen/go/` 子目录被清理

#### Scenario: 包含必要的依赖

- **WHEN** 创建 `api/gen/go.mod`
- **THEN** 必须包含以下依赖：
  - `google.golang.org/protobuf` (protobuf 运行时)
  - `google.golang.org/grpc` (gRPC 运行时)
  - `github.com/go-kratos/kratos/v2` (Kratos 框架)
  - `github.com/envoyproxy/protoc-gen-validate` (验证器)

#### Scenario: import 路径保持不变

- **WHEN** 生成代码位于 `api/gen/go/`
- **THEN** import 路径必须为 `github.com/Servora-Kit/servora/api/gen/go/<path>`
- **THEN** 现有代码的 import 语句需要更新为新的组织路径

