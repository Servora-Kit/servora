# proto-go-package-declaration 规范

## Purpose
待定 - 由归档变更 proposal-1-buf-v2-migration 创建。归档后请更新目的。
## Requirements
### Requirement: 所有 proto 文件必须显式声明 go_package

每个 proto 文件必须包含 `option go_package` 声明，明确指定生成的 Go 代码的 package 路径和别名。

#### Scenario: 添加 go_package 声明

- **WHEN** proto 文件缺少 `option go_package` 声明
- **THEN** 必须添加格式为 `option go_package = "github.com/Servora-Kit/servora/api/gen/go/<path>;<alias>";` 的声明

#### Scenario: go_package 路径格式

- **WHEN** 为 proto 文件添加 `option go_package`
- **THEN** 路径必须遵循格式：`github.com/Servora-Kit/servora/api/gen/go/<proto_path>;<package_alias>`
- **THEN** `<proto_path>` 必须与 proto 文件的目录结构对应（如 `auth/service/v1`）
- **THEN** `<package_alias>` 必须使用服务名 + `pb` 后缀（如 `authpb`, `userpb`, `servorapb`）

#### Scenario: 需要添加 go_package 的文件列表

- **WHEN** 执行迁移
- **THEN** 必须为以下 proto 文件添加 `option go_package`：
  - `api/protos/auth/service/v1/auth.proto` → `authpb`
  - `api/protos/user/service/v1/user.proto` → `userpb`
  - `api/protos/test/service/v1/test.proto` → `testpb`
  - `api/protos/servora/service/v1/i_auth.proto` → `servorapb`
  - `api/protos/servora/service/v1/i_user.proto` → `servorapb`
  - `api/protos/servora/service/v1/i_test.proto` → `servorapb`
  - `api/protos/servora/service/v1/servora_doc.proto` → `servorapb`
  - `api/protos/sayhello/service/v1/sayhello.proto` → `sayhellopb`
  - `api/protos/sayhello/service/v1/sayhello_doc.proto` → `sayhellopb`
  - `api/protos/pagination/v1/pagination.proto` → `paginationpb`

### Requirement: buf.go.gen.yaml 必须简化配置

`buf.go.gen.yaml` 必须删除 `go_package_prefix` 和所有 `override` 配置，依赖 proto 文件中的 `option go_package` 声明。

#### Scenario: 删除 managed mode 配置

- **WHEN** 更新 `buf.go.gen.yaml`
- **THEN** 必须删除 `managed.go_package_prefix` 配置
- **THEN** 必须删除所有 `managed.override` 配置

#### Scenario: 保留必要的 managed 配置

- **WHEN** 更新 `buf.go.gen.yaml`
- **THEN** 必须保留 `managed.enabled: true`
- **THEN** 必须保留 `managed.disable` 列表（禁用外部依赖的 managed mode）

#### Scenario: 更新输出路径

- **WHEN** `buf.go.gen.yaml` 从 `api/` 移到根目录
- **THEN** 所有插件的 `out` 路径必须从 `gen/go` 更新为 `api/gen/go`

### Requirement: 验证生成代码正确性

迁移完成后必须验证生成的代码路径和 package 声明正确。

#### Scenario: 验证生成代码路径

- **WHEN** 执行 `make gen` 生成代码
- **THEN** 生成的文件必须位于 `api/gen/go/<path>/` 目录
- **THEN** 生成的 Go 文件的 package 声明必须与 `option go_package` 中的别名一致

#### Scenario: 验证服务构建

- **WHEN** 执行 `make build`
- **THEN** 所有服务（servora, sayhello）必须能够正常构建
- **THEN** 不得出现 import 路径错误

