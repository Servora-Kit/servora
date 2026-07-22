# AGENTS.md - cmd/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-15 | Updated: 2026-07-17 -->

## Purpose

仓库内 CLI 与代码生成相关命令入口。

## Subdirectories

| Directory | Purpose |
|-----------|---------|
| `svr/` | 统一开发 CLI：`svr new api`、`svr gen gorm`、`svr openfga`（见 `svr/AGENTS.md`） |
| `protoc-gen-typescript-http/` | 仓库内维护的 TypeScript HTTP client 生成器：遵循 canonical ProtoJSON 的 64 位整数字符串映射，并按 `google.api.http` 单段/多段规则编码路径变量 |
| `protoc-gen-go-errors/` | 读取 `servora.errors.v1` 注解并生成 Kratos v3 reason constructor；保持上游兼容的 binary 名称 |
| `protoc-gen-servora-audit/` | 从 `audit_rule`（method）+ `service_default`（service）注解生成审计规则；method 显式字段覆盖 service 默认，未设置字段继承默认 |
| `protoc-gen-servora-authz/` | 从 `rule`（method）+ `service_default`（service）注解生成授权规则；合并语义同 audit |
| `protoc-gen-servora-authn/` | 从 `rule`（method）+ `service_default`（service）注解生成 `AuthnRules() map[string]*authnpb.AuthnRule`；合并语义同 audit/authz；空 schemes 表示使用默认认证 engine 集合 |
| `protoc-gen-servora-crud/` | 从完整 `google.api.resource` 生成 Go/TypeScript 资源名、typed field path 与 descriptor companion；不生成 repository 或事务 |

## For AI Agents

- 从项目根目录运行 `go run ./cmd/svr ...`
- 修改 proto 注解后需重新 `make gen` 以触发对应 plugin
- 各 plugin 的合并测试套件位于对应 `cmd/protoc-gen-servora-*/`；CRUD generator 变更需运行 `go test ./cmd/protoc-gen-servora-crud`、`make gen` 与 `make gen.ts`。
- 修改 `protoc-gen-typescript-http` 后运行 `go test ./cmd/protoc-gen-typescript-http/...`、`make gen.ts`、`make web.typecheck` 和 `make web.build`；生成契约测试位于 `internal/plugin/generate_test.go`
