# go-workspace-config 规范

## Purpose
待定 - 由归档变更 proposal-2-go-modules-workspace 创建。归档后请更新目的。
## Requirements
### Requirement: 必须创建 go.work 聚合所有模块

系统必须在项目根目录创建 `go.work` 文件，聚合所有 Go 模块（根模块、api/gen、servora、sayhello），支持本地多模块开发。

#### Scenario: go.work 包含所有模块

- **WHEN** 在根目录创建 `go.work`
- **THEN** 必须使用 `use` 指令包含以下模块：
  - `.` (根模块)
  - `./api/gen` (生成代码模块)
  - `./app/servora/service` (servora 服务)
  - `./app/sayhello/service` (sayhello 服务)

#### Scenario: go.work 提交到 Git

- **WHEN** 创建 `go.work`
- **THEN** 必须从 `.gitignore` 中移除 `go.work` 忽略规则
- **THEN** `go.work` 必须提交到 Git，确保所有开发者和 CI 使用相同配置

#### Scenario: 本地依赖自动解析

- **WHEN** 存在 `go.work` 并包含所有模块
- **THEN** 服务模块引用 `github.com/Servora-Kit/servora/api/gen` 时必须自动解析到本地 `api/gen` 模块
- **THEN** 服务模块引用 `github.com/Servora-Kit/servora` 时必须自动解析到本地根模块
- **THEN** 不需要在服务 `go.mod` 中添加 `replace` 指令

