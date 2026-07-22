# AGENTS.md - contrib/db/entgo/crud/testdata/entfixture/schema/

<!-- Parent: ../../../AGENTS.md -->
<!-- Updated: 2026-07-21 -->

## 模块目的

本目录定义 CRUD adapter 的中立 Ent live-contract fixture schema；生成输出写入父目录 `entfixture/`，只由 integration test 显式导入。

## 边界约束

- 仅使用 `ContractRow` 与能力型字段名，不承载 User 等业务实体或 repository 语义。
- 只在新增可观察 adapter/方言合同时增加字段。
- 必须复用 `contrib/db/entgo/mixin.SoftDeleteMixin`，不得复制软删除实现。
- Go 测试只消费调用方明确提供的 DSN，不管理数据库或容器生命周期。

## 生成

在 `servora/` 执行：

```bash
go generate ./contrib/db/entgo/crud/testdata/entfixture/schema
```

生成目录不得手工修改，也不得手写 `AGENTS.md`。
