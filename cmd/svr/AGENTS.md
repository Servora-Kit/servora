# AGENTS.md - cmd/svr/

<!-- Parent: ../../AGENTS.md -->
<!-- Generated: 2026-03-15 | Updated: 2026-03-15 -->

## 目录定位

`cmd/svr/` 是仓库内统一开发 CLI，当前支持命令：
- `svr new api <name> <server_name>` - 在指定服务目录下生成 gRPC proto 脚手架
- `svr gen gorm` - GORM GEN 代码生成
- `svr openfga init` - 初始化 OpenFGA store 并上传授权 model
- `svr openfga model apply` - 更新授权 model 到运行中的 OpenFGA 实例

该工具默认假设 **从项目根目录运行**。

## 当前结构

```text
cmd/svr/
├── main.go
└── internal/
    ├── cmd/
    │   ├── gen/
    │   ├── new/
    │   └── openfga/
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
- `<server_name>` 必须对应真实存在的 `app/<server_name>/service` 目录（如 `iam`、`sayhello`）
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

### `svr openfga init`
- 连接 OpenFGA API（默认 `http://localhost:8080`，可通过 `--api-url` 或 `FGA_API_URL` 环境变量配置）
- 查找或创建名为 `servora` 的 store（可通过 `--store` 指定）
- 解析 `.fga` DSL 模型文件并上传为 authorization model
- 自动更新 `.env` 中的 `FGA_API_URL`、`FGA_STORE_ID`、`FGA_MODEL_ID`
- 支持 `--env-prefix`（如 `IAM_`），同时写入带前缀的变量供 Kratos 配置加载使用
- 使用 Go SDK 直连 OpenFGA API，无需 docker/curl/python 等外部依赖

### `svr openfga model apply`
- 将新版本的 `.fga` 模型上传到已有的 store（通过 `--store-id` 或 `FGA_STORE_ID` 环境变量指定）
- 自动更新 `.env` 中的 `FGA_MODEL_ID`（及带前缀版本，若指定 `--env-prefix`）
- 用于权限模型变更后的同步：修改 `servora.fga` → 执行 `svr openfga model apply` → 重启服务

## 当前实现事实

- `main.go` 只调用 `root.Execute()`，失败时 `os.Exit(1)`
- `gen/gorm.go` 定义 4 类失败：`service-not-found`、`config-invalid`、`db-connect-failed`、`generation-failed`
- `discovery.ListAvailableServices()` 依据 `app/*/service` 扫描可用服务

## 常用命令

```bash
go run ./cmd/svr new api billing iam
go run ./cmd/svr new api billing.invoice iam
go run ./cmd/svr gen gorm iam
go run ./cmd/svr gen gorm iam --dry-run
svr openfga init
svr openfga init --api-url http://localhost:8080 --store servora --env-prefix IAM_
svr openfga model apply
svr openfga model apply --store-id <store-id> --model manifests/openfga/model/servora.fga --env-prefix IAM_
```

## 维护提示

- 文档示例必须以项目根目录为基准，不要写成在服务目录执行 `go run ./cmd/svr ...`
- `svr new api` 只生成 proto 骨架，不自动修改服务级生成配置，不自动运行 `make gen`
- `svr gen gorm` 依据 `app/*/service` 扫描可用服务
- `svr openfga` 使用 `openfga/go-sdk` 直连 API + `openfga/language` 解析 DSL，不依赖外部工具
- `svr openfga model apply` 每次上传都会创建新的 model version（OpenFGA 原生版本化），需重启服务使新 `FGA_MODEL_ID` 生效
