# AGENTS.md - api/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-15 | Updated: 2026-03-15 -->

## 目录职责

`api/` 承载三类内容：
- 共享 proto 模块：`api/protos/`
- 统一生成产物：`api/gen/`（Go 在 `go/`，TypeScript 在 `ts/`）
- pnpm workspace 包锚点：`api/ts-client/`

仓库已迁移到 **Buf v2 workspace**：根目录 `buf.yaml` 同时纳管 `api/protos/`、`app/iam/service/api/protos/`、`app/sayhello/service/api/protos/`。

## 当前结构

```text
api/
├── AGENTS.md
├── gen/
│   ├── go.mod        # Go 生成代码独立模块
│   ├── go/           # Go 生成代码（make api 输出）
│   └── ts/           # TypeScript 生成代码（make api-ts 输出，禁止手改）
│       ├── iam/service/v1/
│       ├── authn/service/v1/
│       ├── organization/service/v1/
│       ├── pagination/v1/
│       └── ...
├── ts-client/
│   └── package.json  # pnpm workspace 包 @servora/api-client（仅此一个文件）
└── protos/
    ├── buf.yaml
    ├── buf.lock
    ├── conf/
    └── pagination/
```

## 生成规则

| 命令 | 模板 | 输出 | clean |
|------|------|------|-------|
| `make api` | `buf.go.gen.yaml`（含 authz + mapper + audit 插件） | `api/gen/go/` | true |
| `make api-ts`（共享） | `buf.typescript.gen.yaml` | `api/gen/ts/` | true（每次重建） |
| `make api-ts`（各服务） | `app/*/service/api/buf.typescript.gen.yaml` | `api/gen/ts/` | false（追加） |
| `make openapi` | 各服务 `api/buf.openapi.gen.yaml` | 各服务目录 | — |

> **注意**：共享模板 `clean: true` 先清空 `api/gen/ts/`，服务模板 `clean: false` 追加各自命名空间，因此 `make api-ts` 必须按此顺序执行（Makefile 已保证）。

## 关键文件

- `../buf.yaml`：Buf v2 workspace 配置
- `../buf.go.gen.yaml`：Go 代码生成模板
- `../buf.typescript.gen.yaml`：共享 TS 生成模板（pagination 等）
- `protos/buf.yaml`：共享 proto module 的 lint / breaking 配置
- `ts-client/package.json`：pnpm workspace 包 `@servora/api-client` 的定义文件

## 开发约定

- 共享配置 proto 与跨服务公共 proto 放在 `api/protos/`
- 服务专属业务 proto 放在对应服务的 `app/{service}/service/api/protos/`
- 修改 proto 后运行根目录 `make gen`（Go）或 `make api-ts`（TypeScript）
- **禁止手动编辑** `api/gen/go/` 和 `api/gen/ts/`
- `api/ts-client/` 只有 `package.json`，不要在此存放任何生成或手写代码
- `api/protos/servora/template/service/v1/` 包含 `svr new api` 使用的 proto 模板

## 常用命令

```bash
make api          # 生成 Go 代码 + AuthZ 规则
make api-ts       # 生成所有 TypeScript 客户端（共享 + 各服务）
make openapi      # 生成各服务 OpenAPI 文档
cd api/protos && buf lint
cd api/protos && buf format -w
cd api/protos && buf breaking --against '.git#branch=main'
```
