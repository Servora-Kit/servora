# AGENTS.md - contrib/db/entgo/

<!-- Parent: ../AGENTS.md -->
<!-- Updated: 2026-07-21 -->

## 模块目的

提供 Ent 共享基础设施：组合式 SQL driver、通用 schema mixin，以及 `crud/` 中的类型化 List/Clear adapter。

## 当前结构

```text
contrib/db/entgo/
├── driver.go        # NewDriver(cfg, options...)、WithDB、WithTracing
├── mixin/           # SoftDeleteMixin 与显式 context bypass
└── crud/            # ListFields、List、ClearHelper 与方言 fixture
```

## 当前实现事实

- `NewDriver` 区分 database/sql driver 与 Ent dialect；`WithDB` 借用外部 pool，不取得 Close ownership。
- `SoftDeleteMixin` 提供 tombstone 字段、默认过滤、Delete 改写与显式 bypass，不决定公共 AIP-164 API。
- `crud/` 把 `core/crud.ListQuery` 绑定到 Ent SQL builder；首版没有 GORM CRUD adapter。
- 本级目录负责跨服务 Ent 支撑，不存放具体业务 entity、repository、授权 scope 或事务 runner。

## 边界约束

- 不在这里放具体业务 entity、repository 或查询编排
- `mixin/` 与 `crud/` 属于下级专题目录；基础 driver 不 import CRUD runtime。
- 不把服务私有数据库配置散落到本目录公共代码

## 常见反模式

- 在 `contrib/db/entgo` 中加入只服务于单个业务的 schema 逻辑
- 让 driver 构造依赖服务内部包，破坏共享库边界
- 不把软删除、授权 scope 或业务查询规则复制进基础 driver；具体 scope 始终由 repository 映射。

## 测试与使用

```bash
go test ./contrib/db/entgo/...
SERVORA_ENT_SQLITE_DSN='file:servora_crud_live?mode=memory&cache=shared&_fk=1' make test.ent.sqlite
SERVORA_ENT_POSTGRES_DSN='postgres://…' make test.ent.postgres
```

## 维护提示

- 若修改 `driver.go` 的配置解析方式，需同步检查所有使用 Ent 的服务启动链路
- 若新增 mixin 或 scope 约定，优先保持跨服务可复用，而不是绑定某个业务模型
