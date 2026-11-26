# AGENTS.md - cmd/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-15 | Updated: 2026-07-17 -->

## Purpose

仓库内 Proto 代码生成插件入口。

## Subdirectories

| Directory | Purpose |
|-----------|---------|
| `protoc-gen-redact/` | 读取 `servora.redact.v3` 字段敏感标记注解，生成字段级日志脱敏方法 |
| `protoc-gen-typescript-http/` | 仓库内维护的 TypeScript HTTP client 生成器：遵循 canonical ProtoJSON 的 64 位整数字符串映射，并按 `google.api.http` 单段/多段规则编码路径变量 |
| `protoc-gen-go-errors/` | 读取 `servora.errors.v1` 注解；默认生成 Kratos v3 reason constructor，`target=ts` 生成复用既有联合类型的运行时 reason companion |
| `protoc-gen-servora-conf/` | 从 `servora.conf.v1` 配置注解生成唯一的 `Apply() error` 配置默认与必填处理入口 |

## For AI Agents

- 所有插件均注册在根 `go.mod` 的 `tool` 块中，由 `buf generate` 通过 `go tool` 调用，无需手动 `go install`
- 修改 proto 注解或插件实现后运行 `just gen`（同步刷新 Go 与 TypeScript 生成物）或 `GOWORK=off just gen-check`
- 各 plugin 的测试套件位于对应 `cmd/protoc-gen-*/`
- 修改 `protoc-gen-typescript-http` 后运行 `go test ./cmd/protoc-gen-typescript-http/...`、`just gen-ts`、`just web-typecheck` 和 `just web-build`；生成契约测试位于 `internal/plugin/generate_test.go`
- 修改 `protoc-gen-go-errors` 后运行 `go test ./cmd/protoc-gen-go-errors`、`GOWORK=off go test ./cmd/protoc-gen-go-errors`、`just gen`、`just web-typecheck` 和 `just web-test`
