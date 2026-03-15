# AGENTS.md - api/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-15 | Updated: 2026-03-15 -->

## 目录职责

`api/` 现在主要承载两类内容：
- 共享 proto 模块：`api/protos/`
- 统一 Go 生成产物：`api/gen/`

仓库已迁移到 **Buf v2 workspace**：根目录 `buf.yaml` 同时纳管 `api/protos/`、`app/iam/service/api/protos/`、`app/sayhello/service/api/protos/`。

## 当前结构

```text
api/
├── AGENTS.md
├── gen/
│   ├── go.mod
│   └── go/
└── protos/
    ├── buf.yaml
    ├── buf.lock
    ├── conf/
    └── pagination/
```

## 生成规则

- 根 `Makefile` 的 `make api` 只调用根级 `buf.go.gen.yaml`
- 根 `Makefile` 的 `make api-ts` 直接调用根级固定模板 `buf.typescript.gen.yaml`
- 服务私有 OpenAPI 由各服务目录下的 `api/buf.openapi.gen.yaml` 负责
- 各服务前端 TypeScript 客户端由该服务目录下 `api/buf.typescript.gen.yaml` 输出到该服务 `web/src/service/gen/`（如有）

## 关键文件

- `../buf.yaml`：Buf v2 workspace 配置
- `../buf.go.gen.yaml`：Go 代码生成模板
- `protos/buf.yaml`：共享 proto module 的 lint / breaking 配置
- `gen/go.mod`：生成代码独立模块

## 开发约定

- 共享配置 proto 与跨服务公共 proto 放在 `api/protos/`
- 服务专属业务 proto 优先放在对应服务的 `app/{service}/service/api/protos/`
- 修改 proto 后运行根目录 `make gen`
- 不要手动编辑 `api/gen/go/`
- 当前仓库根目录没有 `buf.typescript.gen.yaml`；直接执行 `make api-ts` 前应先补齐对应模板文件
- `api/protos/template/service/v1/` 包含 `svr new api` 使用的 proto 模板

## 常用命令

```bash
make gen
make api
make openapi
cd api/protos && buf lint
cd api/protos && buf format -w
cd api/protos && buf breaking --against '.git#branch=main'
```

## 注意事项

- 旧文档里提到的 `api/buf.*.go.gen.yaml` 自动扫描规则已过时；当前根级 Go 生成模板是固定文件 `buf.go.gen.yaml`
- IAM / sayhello 的 proto 目录在各自服务的 `api/protos/` 下，共享 proto 在 `api/protos/`
