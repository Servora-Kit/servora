# AGENTS.md - cmd/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-15 | Updated: 2026-03-15 -->

## Purpose

仓库内 CLI 与代码生成相关命令入口。

## Subdirectories

| Directory | Purpose |
|-----------|---------|
| `svr/` | 统一开发 CLI：`svr new api`、`svr gen gorm`、`svr openfga`（见 `svr/AGENTS.md`） |
| `protoc-gen-servora-authz/` | 自定义 protoc 插件，从 proto AuthZ 注解生成授权规则 |

## For AI Agents

- 从项目根目录运行 `go run ./cmd/svr ...`
- 修改 proto 授权注解后需重新 `make api` 以触发 protoc-gen-servora-authz
