# Servora

Proto contract 驱动的 Go 微服务框架；包含运行时、protoc 插件、公共 Proto、根 module 内生成 package 和前端共享包。

## 目录

- `api/protos/`：公共 Proto 与 annotation
- `api/gen/go/`：根 Go module 内的生成 package
- `cmd/`：CLI 与 `protoc-gen-servora-*`
- `security/`：通用 TLS primitive
- `obs/`：日志、追踪、指标、Audit
- `contrib/`：可选第三方 Client 与 capability Adapter
- `web/`：`@servora/proto-utils`

生成目录 `api/gen/go/`、`web/packages/proto-utils/src/gen/` 只由生成命令维护。

## 常用命令

```bash
just gen-fresh      # 清理并生成 Go Proto
just gen-ts         # 生成内建 Proto TypeScript
just lint-proto
just test
just test-all
just web-typecheck
just web-build
just tidy
```

删除或重命名 Proto 后使用 `just gen-fresh`；插件变更先运行 `just plugin`。CI parity 使用 `GOWORK=off`。

## Proto

- package 形如 `servora.<domain>.v1`，目录与 package 对齐。
- `go_package` 形如 `github.com/Servora-Kit/servora/api/gen/go/servora/<domain>/v1`。
- annotation 号段按命名空间从 `5xx00` 起；方法/消息使用 `+0`，服务/字段使用 `+1`。
- 方法级显式字段覆盖服务默认，未显式字段继承默认。

## 发布

Proto 或 Go 生成 package 变化时随根 module 的 `v0.x.y` 一起发布；既有 `api/gen/v*` tag 只保留为历史版本，不再新增。BSR 与 Go module 版本体系相互独立，但根 `v*` Git tag 会触发 Buf CI 自动推送当前 schema，并维护 BSR `main` 与对应版本 label；前端包继续使用 `proto-utils/vx.y.z` tag。

提交格式：`type(scope): description`。

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
