# AGENTS.md - infra/db/ent/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-22 | Updated: 2026-03-22 -->

## 模块目的

提供 Ent 相关的共享基础设施，包括 SQL driver 装配、schema mixin 与 scope 支撑能力。

## 当前结构

```text
infra/db/ent/
├── driver.go
├── mixin/
└── scope/
```

## 当前实现事实

- `driver.go` 根据共享 `conf.Data` 构造 Ent SQL driver
- `mixin/` 当前承载 `timestamp`、`soft_delete` 等通用 schema mixin
- `scope/` 承载 Ent 访问范围相关辅助
- 本级目录负责“Ent 共享支撑”，不是某个具体服务的数据层实现目录

## 边界约束

- 不在这里放具体业务 entity、repository 或查询编排
- `mixin/` 与 `scope/` 属于下级专题目录；本文件只说明一级边界，不递归描述内部实现
- 不把服务私有数据库配置散落到本目录公共代码

## 常见反模式

- 在 `infra/db/ent` 中加入只服务于单个业务的 schema 逻辑
- 让 driver 构造依赖服务内部包，破坏共享库边界
- 把软删除、组织作用域等 mixin 语义复制粘贴到各服务，而不是统一复用

## 测试与使用

```bash
go test ./infra/db/ent/...
```

## 维护提示

- 若修改 `driver.go` 的配置解析方式，需同步检查所有使用 Ent 的服务启动链路
- 若新增 mixin 或 scope 约定，优先保持跨服务可复用，而不是绑定某个业务模型
