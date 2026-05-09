# AGENTS.md - cmd/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-15 | Updated: 2026-03-15 -->

## Purpose

仓库内 CLI 与代码生成相关命令入口。

## Subdirectories

| Directory | Purpose |
|-----------|---------|
| `svr/` | 统一开发 CLI：`svr new api`、`svr gen gorm`、`svr openfga`（见 `svr/AGENTS.md`） |
| `protoc-gen-servora-mapper/` | 从 `mapper` / `mapper_field` 注解生成结构体映射函数 |
| `protoc-gen-servora-audit/` | 从 `audit_rule`（method）+ `service_default`（service）注解生成审计规则；method 显式字段覆盖 service 默认，未设置字段继承默认 |
| `protoc-gen-servora-authz/` | 从 `rule`（method）+ `service_default`（service）注解生成授权规则；合并语义同 audit |
| `protoc-gen-servora-authn/` | 从 `rule`（method）+ `service_default`（service）注解生成认证规则与 `MethodSchemes()` 表；合并语义同 audit；schemes 运行时派发待 P0-4b |

## For AI Agents

- 从项目根目录运行 `go run ./cmd/svr ...`
- 修改 proto 注解后需重新 `make gen` 以触发对应 plugin
- 各 plugin 的合并测试套件位于 `cmd/protoc-gen-servora-{audit,authz,authn}/main_test.go`，新增 service 级字段时务必补合并矩阵 case
