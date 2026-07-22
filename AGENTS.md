# AGENTS.md - servora 框架核心仓库

<!-- Updated: 2026-06-10 -->

## 仓库定位

Servora 是 ProtoBuf-contract-first 的模块化框架库。仓库同时包含 Go 运行时库、protoc 插件、CLI、公共 proto、生成代码模块，以及前端共享包工作区。

主域边界：

| 目录 | 职责 |
| --- | --- |
| `api/protos/` | 公共 proto contract 与 annotation extensions |
| `api/gen/` | 由 `make gen` 生成的独立 Go module；不要手改 |
| `cmd/` | `svr` CLI 与 `protoc-gen-servora-*` 生成器，包括 CRUD generator |
| `web/` | 前端共享包工作区，包含 `@servora/proto-utils` 的 CRUD/client helpers |
| `core/` | bootstrap/config/registry 与后端中立 CRUD runtime/mapper |
| `transport/` | Kratos HTTP/gRPC client/server 装配与通用 middleware |
| `security/` | authn/authz/jwt/tls 等安全基础设施 |
| `obs/` | logger/tracing/metrics/audit 可观测性能力 |
| `contrib/` | kafka/db/k8s/redis/cache 等可选生态集成与 adapter |

隐藏工具目录（`.claude`、`.cursor`、`.opencode`、`.sisyphus`、`.understand-anything`、`.worktrees`、`.venv`）不是框架源码，不要写入架构说明或测试范围。

## 开发约束

提交格式：`type(scope): description`。type：`feat`/`fix`/`refactor`/`docs`/`test`/`chore`。scope 建议使用一级域或二级域：`api`、`cmd`、`web/proto-utils`、`core/bootstrap`、`transport/server`、`security/authn`、`obs/audit`、`contrib/db/redis` 等。

Go 代码保持 `gofmt`/`goimports`，错误返回带上下文，测试优先表驱动。接口保持小；接受 interface、返回具体类型。

## 版本与 tag

- 修改 `core/`、`transport/`、`security/`、`obs/`、`contrib/`、`cmd/`、`api/protos/` 中影响使用者的代码时，主模块打 `git tag v0.x.y`。
- proto 或 `api/gen/` 产物变化时，额外执行 `make tag.api TAG=v0.x.y`，生成 `api/gen/v0.x.y` tag。
- 生成器（`cmd/protoc-gen-servora-*`）改动导致 `api/gen/` 产物变化时，即使 `.proto` 未变，也要打 `api/gen` tag。
- 仅文档、Makefile、CI、基础设施配置变更通常不打 tag。
- 修改 `web/packages/proto-utils/` 并需要发布 npm 时，更新包版本后打 `proto-utils/vx.y.z` tag；这个 tag 只触发 npm 发布 workflow，不触发 Go release。
- `make bsr.push` 会读取 HEAD 上的 `v0.x.y` tag 作为 BSR label；没有 tag 时只推 `main` label。
- 已推送 tag 不要移动。

## Proto contract

命名规则：

- `package` 以 `servora.` 开头并带版本后缀，例如 `servora.audit.v1`。
- 目录与 package 逐段对齐，满足 Buf `PACKAGE_DIRECTORY_MATCH`。
- `go_package` 使用 `github.com/Servora-Kit/servora/api/gen/go/servora/<ns>/v1;<alias>`。
- `import` 相对于 `api/protos/`，例如 `import "servora/audit/v1/annotations.proto";`。

Annotation extension 号段：

| 注解 | 编号 | 消费者 |
| --- | --- | --- |
| `servora.audit.v1.audit_rule` | 50100 | `protoc-gen-servora-audit` |
| `servora.audit.v1.service_default` | 50101 | `protoc-gen-servora-audit` |
| `servora.authz.v1.rule` | 50200 | `protoc-gen-servora-authz` |
| `servora.authz.v1.service_default` | 50201 | `protoc-gen-servora-authz` |
| `servora.authn.v1.rule` | 50300 | `protoc-gen-servora-authn` |
| `servora.authn.v1.service_default` | 50301 | `protoc-gen-servora-authn` |
| `servora.conf.v1.section` | 50400 | `protoc-gen-servora-conf` |
| `servora.conf.v1.field` | 50401 | `protoc-gen-servora-conf` |
| `servora.errors.v1.default_code` | 50500 | `protoc-gen-go-errors` |
| `servora.errors.v1.code` | 50501 | `protoc-gen-go-errors` |

号段约定：每个命名空间从 `5xx00` 起步；`+0` 给 method/message 级，`+1` 给 service/field 级。新增命名空间继续往后递推。

`service_default` 合并语义必须与生成器测试一致：方法级显式字段覆盖服务级默认；未显式字段继承服务级默认；proto3 标量零值无法表达“未设置”，需要优先用 enum/message wrapper。

## 生成与发布流程

新增或修改 proto：

1. 修改 `api/protos/servora/<namespace>/v1/`。
2. 执行 `make lint.proto`。
3. 执行 `make gen`；删除/重命名 proto 或移除 plugin 时用 `make gen.fresh`。
4. 如果前端需要消费内建 proto TS 类型，执行 `make gen.ts`。
5. 执行 `go build ./...` 或更窄的相关测试；涉及 `web/` 时再执行 `make web.typecheck` 和 `make web.build`。
6. 打主 tag；如有 proto/gen 产物变化，执行 `make tag.api TAG=v0.x.y`。
7. BSR 日常推送由 GitHub Actions 处理；`make bsr.push` 仅作本地预演或应急。

不要把业务仓库 proto 放进本仓；业务 proto 放在各自 `app/<svc>/service/api/protos/`。

修改 `@servora/proto-utils`：

1. 修改 `web/packages/proto-utils/`。
2. 如需同步内建 proto TS 类型，执行 `make gen.ts`。
3. 执行 `make web.typecheck` 和 `make web.build`。
4. 发布 npm 时更新 `web/packages/proto-utils/package.json` 版本，并在对应提交上打 `proto-utils/v0.x.y` tag。

## 常用命令

```bash
make init        # 安装 protoc 插件与 CLI 工具
make plugin      # 安装本仓 protoc-gen-servora-* 插件
make gen         # buf generate，增量生成
make gen.fresh   # clean api/gen/go 后重新生成
make gen.ts      # 生成 Servora 内建 proto 的 TypeScript 类型
make web.install # 安装前端共享包依赖
make web.typecheck # typecheck @servora/proto-utils
make web.build   # build @servora/proto-utils
make lint        # Go lint
make ci.lint     # CI 对齐 lint：GOWORK=off + proto lint
make lint.proto  # Buf lint
make test        # go test -short ./...，覆盖主 module 与 api/gen module
make test.all    # 含 integration tag
make tidy        # 两个 module go mod tidy + go work sync
make bsr.push    # 推送 buf.build/servora/servora
```

`GO_WORKSPACE_MODULES := . api/gen`。本仓根 `go.work` 管两个 module；CI parity 场景用 `GOWORK=off`。

## 维护提示

- `api/gen/go/` 只由生成器写入。
- `web/packages/proto-utils/src/gen/` 只由 `make gen.ts` 写入。
- `@servora/proto-utils` 的 npm Trusted Publisher 指向 `Servora-Kit/servora` 与 `.github/workflows/publish-proto-utils.yml`。
- 修改 `cmd/protoc-gen-servora-*` 后运行 `make plugin`，再跑相关 plugin 测试和 `make gen`。
- 生成器输出 shape 改动时，同时检查 `api/gen`、下游示例和本层 tag 规则。
- transport TLS 构造归 `security/tls`，配置 proto 位于 `servora/security/tls/v1/config.proto`，不要把证书解析逻辑散落到 client/server 子包。
- authn/authz 失败/拒绝审计通过 `obs/audit.Auditor` 发送 CloudEvents；安全中间件不要 import audit emitter/recorder 具体实现。
