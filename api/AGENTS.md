# AGENTS.md - api/

<!-- Parent: ../AGENTS.md -->
<!-- Updated: 2026-05-21 -->

## 目录职责

`api/` 承载框架公共 Proto 与根 module 内的 Go 生成 package：

- `api/protos/`：发布到 BSR 的公共 contract，包括 `servora.crud.v1` 与错误注解；
- `api/gen/go/`：`just gen` 写入的 Go package，随 Servora 根 module 发布；
- `just gen-ts`：将内建 Proto TypeScript 类型写入 `web/packages/proto-utils/src/gen`，不在 `api/` 下维护独立 TS package。

## 当前结构

```text
api/
├── AGENTS.md
├── gen/
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
| `just gen` | 执行 `buf generate --template buf.go.gen.yaml`，生成 Go 与 CRUD companion |
| `just gen-ts` | 生成内建 Proto TypeScript 与 CRUD companion 到 proto-utils |
| `just gen-fresh` | 删除 `api/gen/go` 后重新生成；删除/重命名 proto 或移除 plugin 时使用 |
| `just lint-proto` | Buf lint |
| `just fmt-proto` | Buf format |
| `just bsr-update` | 更新 BSR 依赖 |
| `just bsr-push` | 推送 `buf.build/servora/servora`，HEAD 有主 tag 时附加 tag label |

修改 proto 或生成器导致 `api/gen/go` 变化时，先 `just lint-proto && just gen`，再随根 module 的 `v0.x.y` 发布。BSR 发布是独立流程，不因 Go module 发版自动执行。

## 开发约定

- **禁止手动编辑** `api/gen/go/`。
- 公共 proto 放在 `api/protos/servora/<namespace>/v1/`。
- 业务仓库 proto 不放进本仓；各业务服务自行管理自己的 `api/protos/`。
- `api/gen/go` 属于仓库根 module；仓库 CI 使用 `GOWORK=off`，本地父级 `go.work` 只用于可选的跨仓源码联调。
- 生成器输出 shape 改动时，同步检查 `cmd/protoc-gen-servora-*` 测试、`api/gen/go` diff 和下游示例。

## 常见反模式

- 恢复旧的 TS/OpenAPI 生成目录或命令。
- 在 `api/protos/` 下新增模块级 Buf 配置绕过根 `buf.yaml`。
- 只改 proto 不运行 `just gen`。
- 删除/重命名 proto 后仍用增量 `just gen` 留下陈旧生成文件。
