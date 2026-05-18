# AGENTS.md - servora 框架核心仓库

<!-- Updated: 2026-05-12 -->

## 开发约束

### 提交规范

格式：`type(scope): description`。type：`feat`/`fix`/`refactor`/`docs`/`test`/`chore`。scope 建议：`api`/`buf`/`cmd`/`transport`/`security`/`obs`/`core`/`infra`/`repo`，可用"一级域/二级域"结构（如 `security/authn`、`obs/audit`、`core/bootstrap`）。

### 版本管理与打 Tag 规则

- **`git tag v0.x.y`** 打主模块 tag；proto/gen 有变更时额外执行 **`make tag.api TAG=v0.x.y`** 打 `api/gen/v0.x.y`（前缀防呆）
- **生成器边角**：生成器（`cmd/protoc-gen-servora-*` plugin）变更导致 `api/gen/` 产物变更时，**即便 `.proto` 未变，仍需 `make tag.api`**——下游 `go get …/api/gen@…` 才能拿到新生成码
- **何时打 tag**：修改了 `core/`、`transport/`、`security/`、`obs/`、`infra/`、`cmd/`、`api/protos/` 中的代码时（影响 `go get` 或 `buf dep update` 的使用者）
- **何时不打 tag**：仅修改文档、Makefile、CI 配置、基础设施配置等
- BSR label 与 Git tag 自动同步（`make bsr.push` 自动检测 HEAD 上的 tag）
- tag 一旦推送到 remote 就不要移动

## Proto 开发规范

### 命名规范

- `package` 以 `servora.` 开头，带版本后缀（如 `servora.audit.v1`）
- 目录与 `package` 逐段对齐（Buf `PACKAGE_DIRECTORY_MATCH`）
- `go_package`：`github.com/Servora-Kit/servora/api/gen/go/servora/<ns>/v1;<alias>`
- `import`：相对于 `api/protos/`（如 `import "servora/audit/v1/audit.proto";`）

### 注解扩展

| 注解 | 编号 | 消费者 |
|------|-----|--------|
| `servora.mapper.v1.mapper`（MessageOptions） | 50000 | `protoc-gen-servora-mapper` |
| `servora.mapper.v1.mapper_field`（FieldOptions） | 50001 | `protoc-gen-servora-mapper` |
| `servora.audit.v1.audit_rule`（MethodOptions） | 50100 | `protoc-gen-servora-audit` |
| `servora.audit.v1.service_default`（ServiceOptions） | 50101 | `protoc-gen-servora-audit` |
| `servora.authz.v1.rule`（MethodOptions） | 50200 | `protoc-gen-servora-authz` |
| `servora.authz.v1.service_default`（ServiceOptions） | 50201 | `protoc-gen-servora-authz` |
| `servora.authn.v1.rule`（MethodOptions） | 50300 | `protoc-gen-servora-authn` |
| `servora.authn.v1.service_default`（ServiceOptions） | 50301 | `protoc-gen-servora-authn` |
| `servora.conf.v1.section`（MessageOptions） | 50400 | `protoc-gen-servora-conf` |
| `servora.conf.v1.field`（FieldOptions） | 50401 | `protoc-gen-servora-conf` |

号段约定：每命名空间 `5xx00` 起步，`+0` 给 method 级 / message 级，`+1` 给 service 级默认 / field 级；新增命名空间往后递推 `5xx00`。

服务级默认（`service_default`）与方法级注解的合并语义统一为：方法级显式字段覆盖服务级默认；未显式设置的字段继承服务级默认；零值与未设置在 proto3 标量字段下不可区分，因此应优先使用 enum / message 包装类型表达"未设置"。详细规则见各 plugin 文档与 `cmd/protoc-gen-servora-{audit,authz,authn}/` 测试套件。

### 新增 Proto 流程

1. 在 `api/protos/servora/<namespace>/v1/` 下创建 → 2. `make lint.proto` → 3. `make gen` → 4. `go build ./...` → 5. 提交打 tag → 6. `make bsr.push`

### BSR 发布

- 模块名：`buf.build/servora/servora`
- `make bsr.push`：自动检测 HEAD Git tag 作为 BSR label
- 业务仓库通过 `deps: - buf.build/servora/servora` 引用

### 业务仓库 Proto

- 放在各自 `app/<svc>/service/api/protos/`
- `go_package` 使用各自仓库 module 路径
- 不要将业务 proto 放入框架仓库

## 常用命令

```bash
make init                 # 安装 protoc 插件与 CLI 工具
make gen                  # 生成所有代码（增量，日常使用）
make gen.fresh            # 先 clean 再生成（proto 删除/重命名/plugin 移除时使用）
make lint                 # Go lint
make ci.lint              # CI 对齐 lint（GOWORK=off + proto lint）
make lint.proto           # Proto lint
make test                 # 运行测试
make tidy                 # go mod tidy + go work sync
git tag v0.x.y            # 打主模块 tag；proto 有变更时再 make tag.api TAG=v0.x.y
make bsr.push             # 推送 proto 到 BSR
make clean                # 清理生成代码
```

## 联合开发工作流

所有仓库克隆到 `servora-kit/`，顶层 `go.work` 联合管理。修改框架代码后业务仓库立即可见。确认稳定后提交打 tag → 推送 BSR → 业务仓库 `go get` 更新版本。

## 维护提示

- 不要手改 `api/gen/go/`
- 修改 proto 后执行 `make gen`；删除/重命名 proto 或移除 plugin 时用 `make gen.fresh`
- 推送前 `make lint.proto`
- 推送前优先执行 `make ci.lint`，避免本地 `go.work` 对 CI 结果造成误判
- `cmd/protoc-gen-servora-*` 修改后需 `make plugin` 重新安装
