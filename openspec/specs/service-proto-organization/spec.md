# service-proto-organization 规范

## Purpose
待定 - 由归档变更 proposal-3-proto-reorg-build 创建。归档后请更新目的。
## Requirements
### Requirement: 业务 proto 必须移到服务目录

所有业务相关的 proto 文件必须从 `api/protos/` 移动到对应服务的 `proto/` 目录，实现 proto 定义跟随服务。

#### Scenario: servora 服务 proto 移动

- **WHEN** 重组 proto 目录结构
- **THEN** 必须将以下目录从 `api/protos/` 移动到 `app/servora/service/proto/`：
  - `auth/` → `app/servora/service/proto/auth/`
  - `user/` → `app/servora/service/proto/user/`
  - `test/` → `app/servora/service/proto/test/`
  - `servora/` → `app/servora/service/proto/servora/`

#### Scenario: sayhello 服务 proto 移动

- **WHEN** 重组 proto 目录结构
- **THEN** 必须将 `api/protos/sayhello/` 移动到 `app/sayhello/service/proto/sayhello/`

#### Scenario: 框架保留公共 proto

- **WHEN** 重组 proto 目录结构
- **THEN** 以下目录必须保留在 `api/protos/`：
  - `conf/v1/` - 配置定义
  - `pagination/v1/` - 分页公共类型
  - `k8s/` - K8s 相关定义

### Requirement: buf.yaml 必须聚合服务 proto

根目录的 `buf.yaml` 必须更新 `modules` 列表，包含所有服务的 proto 目录。

#### Scenario: 更新 modules 列表

- **WHEN** 更新根 `buf.yaml`
- **THEN** `modules` 列表必须包含：
  - `path: api/protos` (框架公共 proto)
  - `path: app/servora/service/proto` (servora 服务 proto)
  - `path: app/sayhello/service/proto` (sayhello 服务 proto)

#### Scenario: proto 跨引用解析

- **WHEN** servora proto 引用 auth/user/test proto（如 `import "auth/service/v1/auth.proto"`）
- **THEN** Buf v2 workspace 必须能够自动解析引用
- **THEN** 生成代码时不得出现 import 错误

### Requirement: 验证 proto 生成和服务构建

重组完成后必须验证 proto 能够正常生成代码，服务能够正常构建。

#### Scenario: 验证 proto 生成

- **WHEN** 执行 `make gen`
- **THEN** 所有 proto 文件必须能够正常生成代码到 `api/gen/go/`
- **THEN** 不得出现 proto 引用解析错误

#### Scenario: 验证服务构建

- **WHEN** 执行 `make build`
- **THEN** 所有服务（servora, sayhello）必须能够正常构建
- **THEN** 不得出现 import 路径错误
