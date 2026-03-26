# AGENTS.md - infra/openfga/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-22 | Updated: 2026-03-22 -->

## 模块目的

提供 OpenFGA 客户端与常用操作封装，统一组织配置、关系检查、列表查询、tuple 写入、缓存辅助与审计集成。

## 当前文件

- `client.go`：客户端构造、`ClientOption` 模式（`WithAuditRecorder`、`WithComputedRelations`）
- `config.go`：`NewClientOptional` 便捷构造（支持透传 `ClientOption`）
- `check.go`：关系检查封装（`user` 参数为完整 principal，如 `"user:uuid"`）
- `list.go`：列表查询封装（`user` 参数为完整 principal）
- `tuples.go`：tuple 写入/删除（core/public 分层，成功后自动 emit audit 事件）
- `cache.go`：Redis 缓存（`CachedCheck` 返回 `cacheHit`，`InvalidateForTuples` 为 Client 方法）
- `client_test.go`：ClientOption 单元测试
- `cache_test.go`：缓存层去特化单元测试

## 当前实现事实

- `NewClient(cfg, ...ClientOption)` 支持注入 `*audit.Recorder` 和 `ComputedRelationMap`
- `Check`/`ListObjects`/`CachedCheck`/`CachedListObjects` 参数为完整 principal（如 `"user:uuid"`），不再内部拼接前缀
- `CachedCheck` 返回 `(allowed, cacheHit, error)`
- `WriteTuples`/`DeleteTuples` 采用 core/public 分层，成功后自动通过 `obs/audit.Recorder` emit `tuple.changed` 事件
- `InvalidateForTuples` 是 `Client` 方法（需要访问 `computedRelations`）
- `parseTupleComponents` 通用化解析任意 `type:id` principal
- `affectedRelations` 使用 `Client.computedRelations`（通过 `WithComputedRelations` 注入），无硬编码

## 边界约束

- 本包只封装 OpenFGA API 与通用调用模式，不负责策略设计与资源规则建模
- computed relation 映射由调用方通过 `WithComputedRelations` 注入，本包不含任何业务特定映射
- 审计 emit 通过可选的 `*audit.Recorder` 实现，nil-safe
- 不在这里承载 Redis 通用能力；缓存仅是 OpenFGA 场景优化

## 常见反模式

- 在 `infra/openfga` 中硬编码业务资源名、关系名和领域规则
- 把缓存命中逻辑与授权结论语义混为一谈
- 直接在业务层重复拼装 OpenFGA client 而绕过统一 wrapper
- 调用 `Check`/`ListObjects` 时传裸 ID 而非完整 principal

## 测试与使用

```bash
go test ./infra/openfga/...
```

## 维护提示

- 调用方需传完整 principal（如 `"user:" + userID`），不再自动拼接前缀
- `InvalidateForTuples` 现在是 Client 方法，非 package-level 函数
- 若修改配置字段或 client 初始化要求，需同步检查所有服务配置模板
- 若扩展缓存策略，优先保证缓存失效不会放宽授权边界
