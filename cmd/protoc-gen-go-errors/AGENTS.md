# AGENTS.md - cmd/protoc-gen-go-errors/

<!-- Parent: ../AGENTS.md -->
<!-- Updated: 2026-07-30 -->

## Purpose

仓库内维护的同名 `protoc-gen-go-errors`，读取 `servora.errors.v1.default_code` / `code` option。默认 `target=go` 生成 Kratos v3 reason constructor 与 matcher；`target=ts` 复用 TypeScript HTTP generator 的联合类型生成运行时 companion。

## Boundaries

- 缺省 target 与显式 `target=go` 必须生成字节级相同的 `*_errors.pb.go`，且只依赖 `github.com/go-kratos/kratos/v3/errors`。
- `target=ts` 只处理显式声明 Servora error option 的顶层 enum；普通 enum 和 nested enum 不生成 companion。
- TypeScript sidecar 使用 source-relative 的 `*.errors.ts` 文件名，通过 `./index.js` type-only import 复用既有联合类型，并生成同名对象与 `isXxx` guard。
- TypeScript target 不生成 HTTP code、Kratos runtime、message、transport、i18n、用户文案或 UI 行为。
- 两个 target 共享 100..599 HTTP code 校验；无效 option 在生成期报错。
- 插件不定义业务 reason，也不把 storage/business error 归类进框架枚举。

## Verification

在 `servora/` 执行：

```bash
go test ./cmd/protoc-gen-go-errors
GOWORK=off go test ./cmd/protoc-gen-go-errors
just gen
just web-typecheck
just web-test
```
