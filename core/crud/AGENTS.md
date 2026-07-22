# AGENTS.md - core/crud/

<!-- Parent: ../AGENTS.md -->
<!-- Updated: 2026-07-21 -->

## 模块目的

servora 的 crud 生态

`core/crud` 实现后端中立的 CRUD API 生命周期：资源 descriptor Plan、资源名、List 查询预处理、FieldMask、page token 与响应清理。

## 验证

在 `servora/` 执行：

```bash
go test ./core/crud/...
go test -short ./...
```
