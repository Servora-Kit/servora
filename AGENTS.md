# AGENTS.md - servora 框架核心仓库

<!-- Updated: 2026-03-25 -->

## 项目概览

`servora` 是 [Servora-Kit](https://github.com/Servora-Kit) 组织的**核心框架库**，基于 Go Kratos，提供共享基础库（`pkg/`）、自定义 protoc 插件和 CLI 工具（`cmd/`）、框架级公共 Proto 定义（`api/protos/`，发布到 [BSR](https://buf.build/servora/servora)）。

本仓库**不包含业务微服务**。业务位于：`servora-iam`（IAM + 示例）、`servora-platform`（审计等平台服务）。

当前主线事实：
- Go module：`github.com/Servora-Kit/servora`，生成代码：`github.com/Servora-Kit/servora/api/gen`
- 公共 proto 通过 BSR 发布：`buf.build/servora/servora`
- `go.work` 已 gitignore，仅用于本地多仓库联合开发

## 开发约束

### 提交规范

格式：`type(scope): description`。type：`feat`/`fix`/`refactor`/`docs`/`test`/`chore`。scope 建议：`api`/`buf`/`cmd`/`pkg`/`repo`，可用"一级域/二级域"结构（如 `pkg/audit`）。

### 版本管理与打 Tag 规则

- **使用 `make tag TAG=v0.x.y` 自动打双 tag**（`v0.x.y` + `api/gen/v0.x.y`）
- **何时打 tag**：修改了 `pkg/`、`cmd/`、`api/protos/` 中的代码时（影响 `go get` 或 `buf dep update` 的使用者）
- **何时不打 tag**：仅修改文档、Makefile、CI 配置、基础设施配置等
- BSR label 与 Git tag 自动同步（`make buf-push` 自动检测 HEAD 上的 tag）
- tag 一旦推送到 remote 就不要移动

## 顶层目录

- `api/`：公共 proto（`api/protos/servora/`）与 Go 生成代码（`api/gen/go/`）
- `cmd/`：`svr/`（CLI）、`protoc-gen-servora-authz/`、`protoc-gen-servora-audit/`、`protoc-gen-servora-mapper/`、`openapi-merge/`
- `pkg/`：`actor`、`authn`、`authz`、`audit`、`bootstrap`、`broker/kafka`、`cap`、`db/ent`、`governance`、`health`、`helpers`、`jwks`/`jwt`、`k8s`、`logger`、`mail`、`mapper`、`openfga`、`pagination`、`redis`、`swagger`、`transport`

## Proto 开发规范

### 命名规范

- `package` 以 `servora.` 开头，带版本后缀（如 `servora.audit.v1`）
- 目录与 `package` 逐段对齐（Buf `PACKAGE_DIRECTORY_MATCH`）
- `go_package`：`github.com/Servora-Kit/servora/api/gen/go/servora/<ns>/v1;<alias>`
- `import`：相对于 `api/protos/`（如 `import "servora/audit/v1/audit.proto";`）

### 注解扩展

| 注解 | 编号 | 消费者 |
|------|-----|--------|
| `servora.audit.v1.audit_rule` | 50000 | `protoc-gen-servora-audit` |
| `servora.authz.v1.authz_rule` | 50001 | `protoc-gen-servora-authz` |
| `servora.mapper.v1.mapper` | 50002 | `protoc-gen-servora-mapper` |

新增注解编号从 50000 起递增。

### 新增 Proto 流程

1. 在 `api/protos/servora/<namespace>/v1/` 下创建 → 2. `make lint.proto` → 3. `make gen` → 4. `go build ./...` → 5. 提交打 tag → 6. `make buf-push`

### BSR 发布

- 模块名：`buf.build/servora/servora`
- `make buf-push`：自动检测 HEAD Git tag 作为 BSR label
- 业务仓库通过 `deps: - buf.build/servora/servora` 引用

### 业务仓库 Proto

- 放在各自 `app/<svc>/service/api/protos/`
- `go_package` 使用各自仓库 module 路径
- 不要将业务 proto 放入框架仓库

## 常用命令

```bash
make init                 # 安装 protoc 插件与 CLI 工具
make gen                  # 生成所有代码
make lint                 # Go lint
make lint.proto           # Proto lint
make test                 # 运行测试
make tidy                 # go mod tidy + go work sync
make tag TAG=v0.x.y       # 自动打双 tag
make buf-push             # 推送 proto 到 BSR
make clean                # 清理生成代码
```

## 联合开发工作流

所有仓库克隆到 `servora-kit/`，顶层 `go.work` 联合管理。修改框架代码后业务仓库立即可见。确认稳定后提交打 tag → 推送 BSR → 业务仓库 `go get` 更新版本。

## 维护提示

- 不要手改 `api/gen/go/`
- 修改 proto 后执行 `make gen`，推送前 `make lint.proto`
- `cmd/protoc-gen-servora-*` 修改后需 `make plugin` 重新安装
