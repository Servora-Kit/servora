# AGENTS.md - contrib/db/entgo/crud/

<!-- Parent: ../AGENTS.md -->
<!-- Updated: 2026-07-21 -->

## 模块目的

`contrib/db/entgo/crud` 将 `core/crud.ListQuery` 的类型化查询意图绑定到 Ent SQL builder，负责 filter、稳定排序、keyset 分页、skip、可选 Count 与分页令牌游标提取。

## 核心边界

- `ListFields` 只开放 repository 显式声明的查询能力；不得扫描 Ent schema 自动开放字段。
- Adapter 接收调用方提供的 Ent builder，并在其现有连接或事务上执行；不得开启、提交或传播事务。
- 业务 scope 与授权由 biz/data 计算；本包只消费已应用到 builder 的 Ent predicate 与 opaque scope fingerprint。
- PO 到资源 PB 的读映射属于 `core/crud/mapper`；Create/Update 使用具体 repository 的 Ent 原生 setter/mutation。
- Ent/driver 执行错误原样透传；只对 adapter 自身配置、token 和数据合同产出框架错误。

## 实现约束

- `NewListFields` 一次性验证配置并返回不可变对象；请求路径不得重复验证静态 schema 配置。
- 普通列 cursor 使用隐藏 SELECT alias 与实体 `Value(name)` 提取；不得经 PB 或 ResourceMapper 反推。
- nullable 排序固定双向 `NULLS LAST`，order 与 keyset predicate 必须使用同一比较合同。
- SQL 必须通过 Ent `sql.Selector`/dialect builder 生成，不拼接客户端文本。

## 验证

在 `servora/` 执行：

```bash
go test ./contrib/db/entgo/crud/...
go test -short ./...
```
