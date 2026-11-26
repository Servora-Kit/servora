# Servora

Proto contract 驱动的 Go 微服务框架，包含运行时（`core/`、`transport/`、`security/`、`obs/`、`contrib/`）、`protoc` 插件（`cmd/`）、公共 Proto 与生成包（`api/`）以及前端共享包（`web/`）。

## 模块约束

- **`core/` 准入**：仅收容被多个 capability 复用、具备明确协议且与领域解耦的平台能力，新增包在 PR 描述中逐条说明准入依据；单一 capability 的内部辅助代码保留在各自子包内，并按具体职责命名包名。
- **`security/tls` 归口**：服务端与客户端 TLS 配置统一经 `security/tls` 构造 `*crypto/tls.Config`，`transport/`、`obs/` 等调用方直接消费其结果，默认最低版本维持 TLS 1.2。
- **公共 Proto 与生成产物**：`api/protos/servora/<domain>/v1/` 仅存放框架级公共 Proto（业务服务 Proto 由各自业务仓维护），所有 Buf 配置集中于仓库根目录（`buf.yaml`、`buf.lock`、`buf.*.gen.yaml`）。`api/gen/go/` 与 `web/packages/proto-utils/src/gen/` 统一经 `just gen` 更新。
- **`@servora/proto-utils`**：`web/packages/proto-utils/` 保持框架中立，仅承载跨业务复用的传输契约、Kratos 兼容错误合同（`{ code, reason, message, metadata? }`）与内建 Proto TypeScript 类型；HTTP client、认证、重试及前端框架适配由业务仓实现。

## Proto 规范

- package 形如 `servora.<domain>.v1`，目录路径与 package 一致。
- `go_package` 形如 `github.com/Servora-Kit/servora/api/gen/go/servora/<domain>/v1`。
- annotation 号段按命名空间从 `5xx00` 起：方法/消息使用 `+0`，服务/字段使用 `+1`。

## 工具链与常用命令

CLI 版本固定于 `mise.toml`，`protoc` 插件固定于 `go.mod` 的 `tool` 块（由 `buf generate` 经 `go tool` 直接调用），本地 Git 钩子由 `prek.toml` 管理。完整配方见 `justfile`（`just --list`），高频入口如下：

- `just init`：安装固定版本工具链、prek 钩子与前端依赖
- `just gen` / `just gen-fresh`：生成 Go 与内建 Proto TypeScript 代码；删除、重命名 Proto 或调整插件后执行 `just gen-fresh` 全量重建
- `just gen-check`：在 `GOWORK=off` 下校验生成目录零差异
- `just lint` / `just breaking`：依次执行前端、类型、Proto、Go 静态检查 / 检查相对 `origin/main` 的 Proto 兼容性
- `just test` / `just test-all`：运行纯本地 Go 竞态单测 / 运行全量 Go 竞态测试
- `just web-test`：构建、类型检查并运行 `@servora/proto-utils` 单元测试

仓库门禁与 CI 统一在 `GOWORK=off` 下运行，GitHub Actions 以 `ci-ok` 作为唯一必过状态检查。

## 发布与提交

- **根 module 与 BSR**：Proto 或 Go 生成包变更随根 module 的 `v0.x.y` 统一发布（历史 `api/gen/v*` tag 仅作归档保留）。BSR 与 Go module 版本体系相互独立；推送根 `v*` Git tag 时 Buf CI 自动推送当前 schema，并同步维护 BSR `main` 与对应版本 label。
- **前端共享包**：更新 `web/packages/proto-utils/package.json` 版本号并推送 `proto-utils/v<version>` tag，由 GitHub Actions 自动执行 `npm publish --provenance`。
- **提交格式**：`type(scope): description`。

<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **servora** (8115 symbols, 20796 relationships, 287 execution flows).

> Index stale? Run `node .gitnexus/run.cjs analyze --index-only` from the project root — it auto-selects an available runner. No `.gitnexus/run.cjs` yet? Bootstrap with `npx`, `bunx`, or `pnpm dlx` — e.g. `bunx gitnexus@latest analyze` (npm 11 npx crash; #1939).

## Always Do

- **MUST run impact before editing.** Use `impact({target: "symbolName", direction: "upstream"})` or `node .gitnexus/run.cjs impact "symbolName" --direction upstream --repo .`; report callers, processes, and risk. Never substitute grep for graph analysis.
- **MUST analyze graph changes before committing.** Use `detect_changes({scope: "all"})` (MCP) or `node .gitnexus/run.cjs detect-changes --scope all --repo .` (CLI fallback). `partial: true` or `truncated: true` is not a clean check — a zero means unseen, not unaffected; re-run it. For regression review: `detect_changes({scope: "compare", base_ref: "main"})` or `node .gitnexus/run.cjs detect-changes --scope compare --base-ref "main" --repo .`.
- MUST warn on HIGH/CRITICAL `risk` pre-edit; never use `riskSharedAxes` to waive a HIGH/CRITICAL `risk` warning. Compare File/symbol: MCP File omits axes; Graph-RAG expands File.
- **MUST treat `risk: UNKNOWN` as unresolved, not as low.** An empty caller set is not evidence the symbol is unused — it can also mean the callers are not resolvable by the index (plain-object property access, dynamic dispatch, cross-language calls). `impact` pairs `UNKNOWN` with a `riskNote` saying so. Confirm with a text search before treating the symbol as safe to change or delete; do not proceed on the strength of a zero.
- **MUST use `query({search_query: "concept"})` for concepts/flows, `context({name: "symbolName"})` for a named symbol, or `impact` for blast radius, on read-only callers, dependencies, imports, or execution flow.** Graph first; text search only for empty/`UNKNOWN`/literals.
- For security review, `explain({target: "fileOrSymbol"})` lists taint findings (source→sink flows; needs `analyze --pdg`).

## Never Do

- NEVER edit a function, class, or method before MCP/CLI impact analysis.
- NEVER ignore HIGH or CRITICAL risk warnings from impact analysis, and never read `UNKNOWN` as an all-clear — it means the walk could not answer, which is the one verdict that requires confirming by other means.
- NEVER rename symbols with find-and-replace — use `rename` which understands the call graph.
- NEVER commit before MCP/CLI graph change analysis.

## Resources

| Resource | Use for |
| --- | --- |
| `gitnexus://repo/servora/context` | Codebase overview, check index freshness |
| `gitnexus://repo/servora/clusters` | All functional areas |
| `gitnexus://repo/servora/processes` | All execution flows |
| `gitnexus://repo/servora/process/{name}` | Step-by-step execution trace |

## CLI

| Task | Read this skill file |
| --- | --- |
| Understand architecture / "How does X work?" | `.claude/skills/gitnexus-exploring/SKILL.md` |
| Blast radius / "What breaks if I change X?" | `.claude/skills/gitnexus-impact-analysis/SKILL.md` |
| Trace bugs / "Why is X failing?" | `.claude/skills/gitnexus-debugging/SKILL.md` |
| Rename / extract / split / refactor | `.claude/skills/gitnexus-refactoring/SKILL.md` |
| Tools, resources, schema reference | `.claude/skills/gitnexus-guide/SKILL.md` |
| Index, status, clean, wiki CLI commands | `.claude/skills/gitnexus-cli/SKILL.md` |

<!-- gitnexus:end -->
