# AGENTS.md - infra/

<!-- Parent: ../AGENTS.md -->

## 目录定位

`infra/` 是 servora 的「底层外部资源接线层」。所有对外部资源（数据库、消息中间件、缓存、Kubernetes API 等）的 client 接线、连接管理、生命周期与健康探针适配，都集中在这一层。

向上服务：`obs/audit` 用 `infra/broker`（未来）、`core/registry` 用 `infra/k8s` 的 Clientset、各业务用 `infra/redis` 当 cache。

## 准入标准（Admission Gate）

`infra/` 成员 MUST 是**底层外部资源接线**，提供：

- **Client 工厂**（构造连接到外部资源的 client）
- **连接管理**（连接池、重连、关闭）
- **生命周期 hook**（Start/Stop 接口、context 超时传递）
- **健康探针适配**（实现 `core/health.Pinger` 或等价接口）

`infra/` 成员 MUST NOT 混入**使用语义**：

- 不写「业务实体仓库」（如 `UserRepo`、`OrderCache`）
- 不写「业务策略」（如 「缓存命中率优化」、「订单状态机」）
- 不写跨资源的「组合编排」（应该在上层 capability 实现）

使用语义 SHALL 在上层 capability 实现，引用 `infra/` 提供的 client / 连接。

## 当前成员

- `broker/` 消息中间件抽象 + Kafka 实现（franz-go）
- `db/clickhouse/` ClickHouse client 接线
- `db/ent/` ent ORM client 接线（含 mixin / scope helper）
- `k8s/` Kubernetes API client + Pod 自感知（namespace / pod name）
- `redis/` Redis client 接线 + cache / lock 基础语义（仅通用 KV 行为）

## 反模式

- 把业务 entity 的 `Repository` 实现写进 `infra/db/ent/`
- 把业务领域知识（「订单缓存如何穿透」）写进 `infra/redis/`
- 把多个 backend 的「组合策略」（如 「先 cache 后 db」）写进 `infra/`

## 维护提示

- 新增 backend：在 `infra/<category>/<backend>/` 下创建，遵循 client 工厂 + lifecycle + Ping 接口的范式
- 新增类别（如 `infra/queue/` / `infra/storage/`）：先评审是否与现有类别（`broker/` / `db/`）重叠
- 业务侧使用 `infra/` 包时，**业务领域抽象（如 `UserRepository`）应该定义在业务侧**，`infra/` 只提供连接与原生 API 访问
