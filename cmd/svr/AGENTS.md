# AGENTS.md - cmd/svr/

<!-- Generated: 2026-03-09 | Commit: 1f79cd0 -->

## 目录定位

`cmd/svr/` 是仓库内统一开发 CLI，当前支持命令：
- `svr new api <name> <server_name>` - 在指定服务目录下生成 gRPC proto 脚手架
- `svr gen gorm` - GORM GEN 代码生成

该工具默认假设 **从项目根目录运行**。

## 当前结构

```text
cmd/svr/
├── main.go
└── internal/
    ├── cmd/
    │   ├── gen/
    │   └── new/
    ├── discovery/
    ├── generator/
    ├── root/
    ├── scaffold/
    └── ux/
```

## 命令说明

### `svr new api <name> <server_name>`
- 在指定服务目录下生成 gRPC proto 骨架
- `<name>` 必须是小写 snake_case，支持点分层级（如 `billing.invoice`）
- `<server_name>` 必须对应真实存在的 `app/<server_name>/service` 目录
- 输出到 `app/<server_name>/service/api/protos/<name>/service/v1/`
- 只生成 `<name>.proto` 与 `<name>_doc.proto`，不生成 HTTP 专用 `i_*.proto`
- 模板位于 `api/protos/template/service/v1/`
- 生成后需手动运行 `make gen` 生成 Go 代码
- 若需 OpenAPI/TypeScript 生成，需检查服务级 `api/buf.openapi.gen.yaml` 或 `api/buf.typescript.gen.yaml`

### `svr gen gorm`
- 支持多服务参数
- 无参数时进入 `huh` 交互选择
- `--dry-run` 只输出路径，不连数据库
- 批量失败不立即中断，最终统一汇总
- 发现与配置校验逻辑在 `internal/discovery/`

## 当前实现事实

- `main.go` 只调用 `root.Execute()`，失败时 `os.Exit(1)`
- `gen/gorm.go` 定义 4 类失败：`service-not-found`、`config-invalid`、`db-connect-failed`、`generation-failed`
- `discovery.ListAvailableServices()` 依据 `app/*/service` 扫描可用服务

## 常用命令

```bash
go run ./cmd/svr new api billing servora
go run ./cmd/svr new api billing.invoice servora
go run ./cmd/svr gen gorm servora
go run ./cmd/svr gen gorm servora --dry-run
```

## 维护提示

- 文档示例必须以项目根目录为基准，不要写成在服务目录执行 `go run ./cmd/svr ...`
- `svr new api` 只生成 proto 骨架，不自动修改服务级生成配置，不自动运行 `make gen`
- `svr gen gorm` 依据 `app/*/service` 扫描可用服务
