# AGENTS.md - cmd/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-15 | Updated: 2026-07-17 -->

## Purpose

仓库内 CLI 与代码生成相关命令入口。

## Subdirectories

| Directory | Purpose |
|-----------|---------|
| `svr/` | 统一开发 CLI：`svr new api`、`svr gen gorm`（见 `svr/AGENTS.md`） |
| `protoc-gen-typescript-http/` | 仓库内维护的 TypeScript HTTP client 生成器：遵循 canonical ProtoJSON 的 64 位整数字符串映射，并按 `google.api.http` 单段/多段规则编码路径变量 |
| `protoc-gen-go-errors/` | 读取 `servora.errors.v1` 注解；默认生成 Kratos v3 reason constructor，`target=ts` 生成复用既有联合类型的运行时 reason companion |
| `protoc-gen-servora-conf/` | 从 `servora.conf.v1` 配置注解生成唯一的 `Apply() error` 配置默认与必填处理入口 |
| `protoc-gen-servora-crud/` | 从完整 `google.api.resource` 生成 Go/TypeScript 资源名、typed field path 与 descriptor companion；不生成 repository 或事务 |

## For AI Agents

- 从项目根目录运行 `go run ./cmd/svr ...`
- 修改 proto 注解后需重新 `just gen` 以触发对应 plugin
- 各 plugin 的合并测试套件位于对应 `cmd/protoc-gen-servora-*/`；CRUD generator 变更需运行 `go test ./cmd/protoc-gen-servora-crud`、`just gen` 与 `just gen-ts`。
- 修改 `protoc-gen-typescript-http` 后运行 `go test ./cmd/protoc-gen-typescript-http/...`、`just gen-ts`、`just web-typecheck` 和 `just web-build`；生成契约测试位于 `internal/plugin/generate_test.go`
- 修改 `protoc-gen-go-errors` 后运行 `go test ./cmd/protoc-gen-go-errors`、`GOWORK=off go test ./cmd/protoc-gen-go-errors`、`just gen`、`just gen-ts`、`just web-typecheck` 和 `just web-test`。
