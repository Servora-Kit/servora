# AGENTS.md - api/

<!-- Parent: ../AGENTS.md -->
<!-- Updated: 2026-05-21 -->

## 目录职责

`api/` 承载框架公共 Proto 与独立 Go 生成模块：

- `api/protos/`：发布到 BSR 的公共 contract，包括 `servora.crud.v1` 与错误注解；
- `api/gen/`：`make gen` 生成的独立 Go module；
- `make gen.ts`：将内建 Proto TypeScript 类型写入 `web/packages/proto-utils/src/gen`，不在 `api/` 下维护独立 TS package。

## 当前结构

```text
api/
├── AGENTS.md
├── gen/
│   ├── go.mod
│   └── go/                 # buf.go.gen.yaml 输出，禁止手改
└── protos/
    ├── AGENTS.md
    ├── README.md           # BSR 展示用
    └── servora/            # package root
```

Buf 配置在仓库根：`buf.yaml`、`buf.lock`、`buf.go.gen.yaml`。`api/protos/` 下没有独立 `buf.yaml` 或 `buf.lock`。

## 生成与发布

| 命令 | 作用 |
| --- | --- |
| `make gen` | 执行 `buf generate --template buf.go.gen.yaml`，生成 Go 与 CRUD companion |
| `make gen.ts` | 生成内建 Proto TypeScript 与 CRUD companion 到 proto-utils |
| `make gen.fresh` | 删除 `api/gen/go` 后重新生成；删除/重命名 proto 或移除 plugin 时使用 |
| `make lint.proto` | Buf lint |
| `make fmt.proto` | Buf format |
| `make bsr.update` | 更新 BSR 依赖 |
| `make tag.api TAG=v0.x.y` | 创建 `api/gen/v0.x.y` tag |
| `make bsr.push` | 推送 `buf.build/servora/servora`，HEAD 有主 tag 时附加 tag label |

修改 proto 或生成器导致 `api/gen/go` 变化时，先 `make lint.proto && make gen`，再按根文档的主 tag + `make tag.api` 规则发布。

## 开发约定

- **禁止手动编辑** `api/gen/go/`。
- 公共 proto 放在 `api/protos/servora/<namespace>/v1/`。
- 业务仓库 proto 不放进本仓；各业务服务自行管理自己的 `api/protos/`。
- `api/gen/go.mod` 是独立 module；根 `go.work`/Makefile 同时覆盖 `.` 与 `api/gen`。
- 生成器输出 shape 改动时，同步检查 `cmd/protoc-gen-servora-*` 测试、`api/gen/go` diff 和下游示例。

## 常见反模式

- 恢复旧的 TS/OpenAPI 生成目录或命令。
- 在 `api/protos/` 下新增模块级 Buf 配置绕过根 `buf.yaml`。
- 只改 proto 不运行 `make gen`。
- 删除/重命名 proto 后仍用增量 `make gen` 留下陈旧生成文件。
