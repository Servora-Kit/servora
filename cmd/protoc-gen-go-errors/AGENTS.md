# AGENTS.md - cmd/protoc-gen-go-errors/

<!-- Parent: ../AGENTS.md -->
<!-- Updated: 2026-07-21 -->

## Purpose

仓库内维护的同名 `protoc-gen-go-errors`，读取 `servora.errors.v1.default_code` / `code` option，为错误枚举生成 Kratos v3 reason constructor 与 matcher。

## Boundaries

- 输出只依赖 `github.com/go-kratos/kratos/v3/errors`，不得生成 Kratos v2 import。
- 只处理显式声明 Servora error option 的 enum；普通 enum 不生成文件。
- HTTP code 必须处于有效范围；错误在生成期暴露。
- 生成文件名为源 Proto 的 `*_errors.pb.go`，遵守 `paths=source_relative`。
- 插件不定义业务 reason，也不把 storage/business error 归类进框架枚举。

## Verification

在 `servora/` 执行：

```bash
go test ./cmd/protoc-gen-go-errors
make plugin
make gen
```
