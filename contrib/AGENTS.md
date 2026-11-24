# contrib

Servora 可选第三方生态空间，包含 database、cache 与 Kubernetes runtime。

## 组织边界

- base package 负责官方 Client 构造、配置映射、生命周期、显式 provider-native 日志/tracing/健康检查能力与 helper
- Redis provider 只接收 Proto 配置：`redis.New(cfg)` 返回官方 client 与 cleanup，不接收业务 logger
- capability Adapter 位于对应能力路径，例如 Ent CRUD 位于 `contrib/db/entgo/crud`，Redis cache strategy 位于 `contrib/cache/redis`
- `contrib/db/redis` 提供 Redis Client、lock 与 KV helper
- Provider error 使用稳定类型和 `errors.Is/As` 分类

公共 Proto config 变更后运行 `just gen-fresh`、`just gen` 和 Proto lint。
