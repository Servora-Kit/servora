# AGENTS.md - servora 框架核心仓库

<!-- Generated: 2026-03-15 | Updated: 2026-03-25 -->

## 项目概览

`servora` 是 [Servora-Kit](https://github.com/Servora-Kit) 组织的**核心框架库**，基于 Go Kratos，提供共享基础库（`pkg/`）、自定义 protoc 插件和 CLI 工具（`cmd/`）、框架级公共 Proto 定义（`api/protos/`，发布到 [BSR](https://buf.build/servora/servora)）。

本仓库**不包含业务微服务实现**。业务微服务位于独立仓库：
- `servora-iam`：IAM 与示例微服务（含前端）
- `servora-platform`：平台级基础服务（审计等）

当前主线事实：
- 所有开发均在 `main` 分支进行
- Go module 路径：`github.com/Servora-Kit/servora`
- 生成代码 module：`github.com/Servora-Kit/servora/api/gen`
- 公共 proto 通过 BSR 发布：`buf.build/servora/servora`
- `go.work` 已 gitignore，仅用于本地多仓库联合开发

## 开发约束

### 提交消息格式

**强制规范**：所有 Servora-Kit 组织下仓库（servora、servora-iam、servora-platform）的提交必须遵循以下格式：

```
type(scope): description
```

**允许的 type**：`feat`、`fix`、`refactor`、`docs`、`test`、`chore`

**建议的 scope**：
- `api`：API / Proto / OpenAPI 相关
- `buf`：Buf 配置与生成链路
- `cmd`：CLI 工具
- `pkg`：框架核心代码
- `repo`：仓库治理/元信息（如 ignore、目录约定）

> 说明：scope 不必来自上述列表，只校验 `type(scope): description` 基本格式。
> scope 仍建议使用小写、语义化、简短命名（可包含 `a-z`、`0-9`、`.`、`_`、`/`、`-`）。
> 推荐优先采用"一级域/二级域"结构，例如：`pkg/audit`、`cmd/svr`、`api/proto`。

**提交最佳实践**：
1. 保持提交小而专注：一个提交只做一件事
2. 使用清晰的描述：描述"做了什么"，而不是"怎么做的"
3. 遵循格式：保持 `type(scope): description` 格式便于历史与工具解析

### 版本管理

- Go module 版本通过 Git tag 发布：`v0.1.1`、`v0.2.0` 等
- `api/gen` 子模块独立打 tag：`api/gen/v0.1.1` 等
- BSR 发布版本与 Git tag 保持一致
- 业务仓库通过 `go get github.com/Servora-Kit/servora@<version>` 引用

## 顶层目录

- `api/`：框架级公共 proto（`api/protos/`）与 Go 生成代码（`api/gen/go/`）
- `cmd/`：CLI 工具与自定义 protoc 插件
  - `svr/`：中心化 CLI（`svr gen gorm`、`svr openfga`）
  - `protoc-gen-servora-authz/`：AuthZ 规则生成插件
  - `protoc-gen-servora-audit/`：Audit 注解生成插件
  - `protoc-gen-servora-mapper/`：对象映射生成插件
  - `openapi-merge/`：OpenAPI 合并工具
- `pkg/`：共享基础库
  - `actor`（通用 principal 模型：Subject/Roles/Scopes/Attrs/ServiceActor）
  - `authn`（可插拔认证：JWT / Header / Noop）
  - `authz`（可插拔授权：OpenFGA / Noop）
  - `audit`（全链路审计：Emitter/Recorder/middleware）
  - `bootstrap`（服务启动引导，含配置重载 loader）
  - `broker` / `broker/kafka`（消息代理抽象 + franz-go 实现）
  - `cap`
  - `db/ent`（Ent schema mixin 与 scope 工具）
  - `governance`（服务治理：注册发现、配置中心）
  - `health`（健康检查）
  - `helpers`（通用工具函数）
  - `jwks` / `jwt`（JWT 签发与 JWKS 验证）
  - `k8s`（K8s 客户端工具）
  - `logger`（日志封装：New/For/Zap）
  - `mail`（邮件发送）
  - `mapper`（对象映射）
  - `openfga`（OpenFGA 客户端封装与缓存）
  - `pagination`（分页工具）
  - `redis`（Redis 客户端封装）
  - `swagger`（Swagger UI 集成）
  - `transport`（HTTP/gRPC 传输层，含 middleware chain、WhiteList、IdentityFromHeader、TokenFromContext）

## 关键文件

- `Makefile`：框架构建入口（init / gen / api / lint / test / compose / buf-push）
- `buf.yaml`：Buf v2 workspace，公共 proto 模块名 `buf.build/servora/servora`
- `buf.go.gen.yaml`：Go 代码生成模板（含自定义 authz / audit / mapper 插件）
- `go.mod`：根 Go module
- `go.work`（gitignored）：本地多仓库联合开发用

## 公共 Proto 开发规范

### 当前目录结构

```text
api/protos/servora/
├── audit/v1/
│   ├── annotations.proto   # 审计注解扩展（AuditRule, MethodOptions 扩展）
│   └── audit.proto         # 审计事件类型枚举
├── authz/v1/
│   └── authz.proto         # 授权注解扩展（AuthzRule, AuthzMode, MethodOptions 扩展）
├── conf/v1/
│   ├── conf.proto          # 共享配置结构（Bootstrap, Server, Data, Registry, Tracer...）
│   └── config-example.yaml # 配置文件示例
├── mapper/v1/
│   └── mapper.proto        # 对象映射注解
└── pagination/v1/
    └── pagination.proto    # 分页请求/响应消息（PagePagination, CursorPagination）
```

### Proto 命名规范

1. **package 命名**：必须以 `servora.` 开头，显式携带版本后缀
   - 正确：`package servora.audit.v1;`
   - 错误：`package audit.v1;`（缺少 `servora.` 前缀）

2. **目录对齐**：`.proto` 文件所在目录必须与 `package` 命名空间逐段对齐
   - `servora.audit.v1` → `servora/audit/v1/`
   - 必须满足 Buf lint 规则 `PACKAGE_DIRECTORY_MATCH`

3. **go_package 格式**：统一落到 `api/gen/go/servora/**`，使用 `;别名` 后缀
   - 格式：`option go_package = "github.com/Servora-Kit/servora/api/gen/go/servora/<namespace>/v1;<alias>";`
   - 示例：`option go_package = "github.com/Servora-Kit/servora/api/gen/go/servora/audit/v1;auditv1";`
   - 别名命名：`<namespace>v1`（如 `auditv1`）或 `<namespace>pb`（如 `authzpb`、`paginationpb`）

4. **import 路径**：使用相对于 proto 根（`api/protos/`）的路径
   - 正确：`import "servora/audit/v1/audit.proto";`
   - 错误：`import "api/protos/servora/audit/v1/audit.proto";`

### 注解扩展约定

框架提供三类 MethodOptions 注解扩展（编号分配参见各 proto 文件）：

| 注解 | 扩展编号 | 消费者 | 用途 |
|------|---------|--------|------|
| `servora.audit.v1.audit_rule` | 50000 | `protoc-gen-servora-audit` | RPC 方法审计规则 |
| `servora.authz.v1.authz_rule` | 50001 | `protoc-gen-servora-authz` | RPC 方法授权规则 |
| `servora.mapper.v1.mapper` | 50002 | `protoc-gen-servora-mapper` | 对象映射规则 |

业务 proto 通过 `import` 引入注解，并在 RPC 方法上使用 `option (servora.xxx.v1.xxx) = { ... };` 标注。

### 新增公共 Proto 流程

1. 在 `api/protos/servora/<namespace>/v1/` 下创建 `.proto` 文件
2. 遵循上述命名规范设置 `package`、`go_package`
3. 如果是新的注解扩展，分配唯一的 MethodOptions 扩展编号（50000 起）
4. 执行 `make lint.proto` 确保通过 Buf lint
5. 执行 `make gen` 生成 Go 代码
6. 验证构建：`go build ./...`
7. 提交后打 tag 并推送 BSR：`make buf-push`

### Buf 配置

- `buf.yaml`：声明模块名 `buf.build/servora/servora`，依赖 googleapis、kratos/apis、protovalidate、gnostic
- `buf.go.gen.yaml`：Go 代码生成模板，包含标准 protoc 插件和三个 servora 自定义插件
- Buf lint 规则使用 `STANDARD`，豁免 `ENUM_VALUE_PREFIX`、`ENUM_ZERO_VALUE_SUFFIX`、`RPC_REQUEST_RESPONSE_UNIQUE`

### BSR 发布

- 模块名：`buf.build/servora/servora`
- 发布命令：`make buf-push`
  - 自动检测 HEAD 上的 Git tag（`vX.Y.Z` 格式）
  - 有 tag 时作为 BSR label 推送；无 tag 时推送但不设 label
- BSR label 与 Git tag 自动保持一致，无需手动指定
- 业务仓库通过 `buf.yaml` 中的 `deps: - buf.build/servora/servora` 引用，`buf dep update` 拉取

### 业务仓库 Proto 规范

业务仓库（如 servora-iam）中的 proto：
- 放在各自的 `app/<service>/service/api/protos/` 下
- `go_package` 使用**各自仓库**的 module 路径（如 `github.com/Servora-Kit/servora-iam/api/gen/go/...`）
- 通过 `import` 引用本仓库的公共注解（BSR 自动解析）
- 不要将业务 proto 放入框架仓库

## 常用命令

```bash
make init          # 安装 protoc 插件与 CLI 工具
make gen           # 生成所有代码（api）
make api           # 仅生成 proto Go 代码
make lint          # Go lint
make lint.proto    # Proto lint
make test          # 运行测试
make tidy          # go mod tidy + go work sync
make buf-push      # 推送 proto 到 BSR
make clean         # 清理生成代码
```

## 多仓库联合开发工作流

1. 所有 Servora-Kit 仓库克隆到同一父目录（如 `servora-kit/`）
2. 父目录的 `go.work` 纳管所有模块，通过 `replace` 指令实现本地引用
3. 修改框架代码后，业务仓库立即可见，无需发版
4. 修改公共 proto 后：
   - 在 `servora` 目录执行 `make gen` 重新生成框架 Go 代码
   - 在业务仓库目录执行 `make gen` 重新生成业务 Go 代码
5. 确认稳定后，提交框架仓库并打 tag → 推送 BSR → 业务仓库更新 `go.mod` 版本号

## 维护提示

- `make api` 使用 `buf.go.gen.yaml`（含自定义插件）
- 修改 proto 后执行 `make gen`
- 不要手改 `api/gen/go/`
- 修改后推送 BSR 前先 `make lint.proto` 确保通过
- 自定义 protoc 插件代码在 `cmd/protoc-gen-servora-*`，修改后需 `make plugin` 重新安装
