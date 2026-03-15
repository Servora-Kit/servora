# AGENTS.md - app/iam/service/

<!-- Parent: ../../AGENTS.md -->
<!-- Generated: 2026-03-15 | Updated: 2026-03-15 -->

## 目录定位

`app/iam/service/` 是 IAM 服务模块，包含：
- 服务私有 proto：`api/protos/`（含 authn、authz、organization、project 等）
- Go 后端：`cmd/` + `internal/`
- OpenAPI 产物：`openapi.yaml`（由 buf 生成）

独立 Go module，受根 `go.work` 管理。

## 当前结构

```text
app/iam/service/
├── api/
│   ├── buf.openapi.gen.yaml
│   └── protos/           # authn, authz, organization, project 等
├── cmd/                  # 启动入口
├── configs/              # 配置
├── internal/
│   ├── biz/              # UseCase 与仓储接口
│   ├── data/             # Ent / GORM 数据层
│   ├── oidc/             # OIDC 登录等
│   ├── server/           # Server 与中间件装配
│   └── service/          # gRPC / HTTP 接口实现
├── manifests/
├── go.mod
├── Makefile
└── openapi.yaml
```

## 代码层次

- `internal/biz/`：业务用例与仓储接口
- `internal/data/`：Ent / GORM 实现
- `internal/oidc/`：OIDC 登录流程
- `internal/service/`：gRPC / HTTP 协议适配
- `internal/server/`：Server 与中间件装配

## Proto / OpenAPI

- 业务 proto 位于 `api/protos/`，根 `make api` 统一生成到 `api/gen/go/`
- OpenAPI 模板：`api/buf.openapi.gen.yaml`

## 常用命令

```bash
make gen
make build
make run
make wire
make gen.ent
make gen.gorm
make openapi
```

## 维护提示

- 修改 proto 后执行根目录或本目录 `make gen`
- 不要手动编辑 `openapi.yaml`、`wire_gen.go`、`api/gen/`
