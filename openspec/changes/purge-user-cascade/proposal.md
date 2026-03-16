# PurgeUser 级联删除原子性与僵尸数据

## 动机

`PurgeUser` 当前跨 FGA、Redis、Postgres 三步执行，无分布式事务。DB 内 `PurgeCascade` 已用 `InTx` 保证原子，但存在：

1. **执行顺序**：当前为「先 FGA 再 Redis 再 DB」。若服务在任两步之间崩溃，会出现跨系统不一致（例如 DB 已删用户、FGA 仍留 tuple，或反之）。
2. **孤儿数据**：`PurgeCascade` 只删 `OrganizationMember`、`ProjectMember`、`User`，未删该用户作为 **owner** 的 Organization 及其下属 Project，会留下孤儿行。

多租户下更需保证用户删除后不留下跨系统脏数据与孤儿资源。

## 目标

1. 明确 PurgeUser 的跨系统顺序或补偿策略，避免「DB 已删、FGA/Redis 仍留」或「FGA 已删、DB 未删」的不一致。
2. 在 PurgeCascade 同一事务内，按依赖顺序删除该用户拥有的 Organization、Project（在删除 User 前），避免孤儿行。
3. 可选：打点/日志便于排查中断点；提供按 `user_id` 清理 FGA/Redis 残留的补偿脚本或管理接口。

## Capabilities

- **purge-user-order**：PurgeUser 执行顺序或补偿策略（先 DB 再 FGA 再 Redis，或补偿/重试）
- **purge-cascade-owned**：PurgeCascade 内删除用户拥有的 Organization、Project
- **purge-compensation**（可选）：按 user_id 清理 FGA/Redis 残留的脚本或接口

## 非目标

- 不引入分布式事务（2PC/Saga）；仅通过顺序 + 单系统事务 + 可选补偿降低不一致窗口。
- 不改变 PurgeUser 的 API 契约（仍为物理删除用户及其关联成员关系）。
