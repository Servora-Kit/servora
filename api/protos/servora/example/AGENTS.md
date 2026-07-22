# AGENTS.md - api/protos/servora/example/

<!-- Parent: ../../AGENTS.md -->
<!-- Updated: 2026-07-22 -->

## 当前定位

`api/protos/servora/example/` 保存 Servora CRUD 生态唯一公开参考资源的 Proto 契约。它用于验证生成器、runtime、Ent adapter 与 reference application 的组合，不代表通用业务模型。

## 核心约束

- `example.servora.dev/User` 是唯一公开参考资源，canonical pattern 为 `tenants/{tenant}/users/{user}`。
- `User` 的资源名、字段行为、AIP-164 Delete/Undelete 与 List 扩展必须保持与参考应用和 OpenSpec CRUD 合同一致。
- `temporary_password` 只能是 `INPUT_ONLY` 公共输入；hash、授权、租户 scope 和事务不进入 Proto。
- 新增字段或 RPC 后必须重新生成 Go/TypeScript 产物，并同步 reference application 与 contract checks。
- 不在此目录放生成代码、业务 service、Ent schema 或 repository 实现。

## 验证

在 `servora/` 执行：

```bash
make lint.proto
make gen
```
