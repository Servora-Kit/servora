# AGENTS.md - app/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-15 | Updated: 2026-03-15 -->

## 目录概览

`app/` 存放可运行服务。当前仓库内有两个服务模块：
- `app/iam/service/`：IAM 主服务（认证、授权、组织、项目），含后端与可选前端
- `app/sayhello/service/`：独立示例服务

每个服务目录都是独立 Go module，并通过根 `go.work` 纳管。

## 服务共同结构

```text
app/{service}/service/
├── api/         # 服务私有 proto、TS/OpenAPI 生成模板
├── cmd/         # 启动入口
├── configs/     # 配置
├── internal/    # 实现代码
├── Makefile     # include ../../../app.mk
└── go.mod       # 独立模块
```

各服务目录可包含：
- `web/`：前端（如有）
- `manifests/`：服务专属补充资源
- `openapi.yaml`：服务 OpenAPI 产物（由 buf 生成）

## 关键约定

- 服务目录中的 `make gen` 会执行 `wire + api + openapi + gen.ent`
- 服务目录中的 `make build` 会先执行 `make gen`，再编译当前服务
- 服务目录中的 `make api` 会回到仓库根目录跑 `make api-go`
- 若存在 `api/buf.typescript.gen.yaml`，服务级 `make api` 会额外生成 TypeScript 客户端
- 服务级 `make openapi` 读取本目录 `api/buf.openapi.gen.yaml`

## 常用命令

```bash
cd app/iam/service && make run
cd app/iam/service && make build
cd app/iam/service && make wire
cd app/iam/service && make gen.ent
cd app/iam/service && make gen.gorm
cd app/sayhello/service && make run
```

## 维护提示

- 部署清单以根 `manifests/` 为主；各服务可带 `manifests/` 补充资源
- 若新增服务，优先参考 `app/sayhello/service/` 的最小结构，再按需要补齐 `api/` 与 `internal/`
