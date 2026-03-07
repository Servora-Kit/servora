## Purpose
定义 svr-new-api-command 的功能需求和验证场景。

## Requirements

### Requirement: 服务维度的新建 proto 命令
系统必须提供 `svr new api <name> <server_name>` 命令，用于在指定服务目录下生成新的 gRPC proto 骨架。

#### Scenario: 在 servora 中创建业务 proto
- **WHEN** 用户执行 `svr new api billing servora`
- **THEN** 系统必须在 `app/servora/service/api/protos/billing/service/v1/` 创建目标目录
- **THEN** 系统必须生成 `billing.proto` 与 `billing_doc.proto`

#### Scenario: 在 sayhello 中创建业务 proto
- **WHEN** 用户执行 `svr new api inventory sayhello`
- **THEN** 系统必须在 `app/sayhello/service/api/protos/inventory/service/v1/` 创建目标目录
- **THEN** 系统必须生成 `inventory.proto` 与 `inventory_doc.proto`

### Requirement: 服务名必须有效
命令必须校验 `server_name` 对应一个真实存在的 `app/<server_name>/service` 目录；禁止向不存在的服务写入 proto。

#### Scenario: 服务存在
- **WHEN** 用户执行 `svr new api billing servora`
- **THEN** 系统必须通过服务目录校验并继续生成流程

#### Scenario: 服务不存在
- **WHEN** 用户执行 `svr new api billing notfound`
- **THEN** 系统必须输出服务不存在的错误信息
- **THEN** 系统必须以非零退出码结束
- **THEN** 系统不得创建任何目录或文件

### Requirement: 只生成 gRPC proto 骨架
该命令必须只生成通用 gRPC proto 与 doc proto，禁止自动生成 HTTP 专用 `i_*.proto` 文件。

#### Scenario: 成功生成后文件集合固定
- **WHEN** 用户成功执行 `svr new api billing servora`
- **THEN** 输出文件必须仅包含 `billing.proto` 与 `billing_doc.proto`
- **THEN** 系统不得额外生成 `i_billing.proto`

### Requirement: 命名规则必须延续现有 proto 约定
命令必须继续要求 `<name>` 使用小写 snake_case，并允许点分层级，以满足 proto 包名与目录结构约定。

#### Scenario: 合法 snake_case 名称
- **WHEN** 用户执行 `svr new api say_hello servora`
- **THEN** 系统必须正常生成 `say_hello.proto` 与 `say_hello_doc.proto`

#### Scenario: 合法点分层级名称
- **WHEN** 用户执行 `svr new api billing.invoice servora`
- **THEN** 系统必须生成到 `app/servora/service/api/protos/billing/invoice/service/v1/`
- **THEN** 文件名必须为 `billing_invoice.proto` 与 `billing_invoice_doc.proto`

#### Scenario: 非法名称
- **WHEN** 用户执行 `svr new api Billing servora`
- **THEN** 系统必须输出名称格式错误信息
- **THEN** 系统必须以非零退出码结束

### Requirement: 目标目录冲突必须阻止覆盖
命令必须在写入前检查目标目录是否已存在；目标目录已存在时禁止覆盖。

#### Scenario: 目标目录已存在
- **WHEN** `app/servora/service/api/protos/billing/service/v1/` 已存在且用户执行 `svr new api billing servora`
- **THEN** 系统必须输出冲突错误
- **THEN** 系统必须以非零退出码结束
- **THEN** 系统不得修改已有文件

## 修改需求

## 移除需求
