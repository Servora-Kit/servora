# 设计文档：Servora 接入 Keycloak 后的认证、授权、审计与框架演进

**日期：** 2026-03-20
**最后更新：** 2026-03-23
**状态：** Phase 1–2b 已完成 · Phase 3（all-in-proto codegen）可启动

---

## 进度总览

| 阶段 | 名称 | 状态 | OpenSpec |
|------|------|------|---------|
| Phase 1 | 框架骨架 (framework-audit-skeleton) | ✅ 已完成 | `openspec/changes/archive/2026-03-20-framework-audit-skeleton/` |
| Phase 2a | 审计 emit 接入（pkg 层） | ✅ 已完成 | `openspec/changes/archive/2026-03-22-audit-emit-integration/` |
| Phase 2b | Audit Service + ClickHouse | ✅ 已完成 | `openspec/changes/archive/2026-03-20-audit-service-clickhouse/` |
| Phase 3 | all-in-proto 代码生成 | 📋 规划中 | — |
| Phase 4 | Keycloak 接入 | 📋 规划中 | — |
| Phase 5 | Servora 生态扩展 | 📋 规划中 | — |

**已沉淀 specs（16 个）：** `openspec/specs/` 下的 actor-v2、audit-clickhouse-storage、audit-kafka-consumer、audit-proto、audit-query-api、audit-runtime、audit-service-scaffold、authz-audit-emit、broker-abstraction、config-proto-extension、identity-header-enhancement、infra-kafka-clickhouse、logger-refactor、openfga-audit-emit、openfga-framework-api、proto-package-governance。

---

## 1. 背景与目标

Servora 当前仓库同时承载了框架能力（`pkg/`、`cmd/`、`api/`）与一个早期自建 IAM 服务实现。现阶段已经明确：

- 未来希望将 **Servora 打造成面向微服务快速开发的脚手架与框架生态**；
- `pkg/` 中的能力会逐步框架化、通用化，并最终作为 **Servora 生态 Go 包** 对外发布；
- 认证引入 **Keycloak**，授权继续采用 **OpenFGA**，审计采用 **Kafka + ClickHouse**；
- 当前 IAM 和 sayhello 服务已从工具链（Makefile、docker-compose.dev）中移除，保留代码作为未来新服务的参考模板。

---

## 2. 核心决策（不变）

| 决策点 | 结论 |
|---|---|
| 认证中心 | 使用 **Keycloak** |
| 网关认证策略 | 由 **Traefik / Gateway 统一验 token** |
| 业务服务是否重复验 JWT | 默认 **不重复验**，优先信任网关注入的 principal header |
| 授权底座 | 继续使用 **OpenFGA** |
| 授权执行位置 | **各业务服务本地** 接入 `pkg/authz` |
| 是否保留中央 IAM/AuthZ 在线代理 | **不保留**；最多保留薄的管理/后台能力 |
| 审计架构 | **中心化 Audit Service + 非中心化 authz/audit emit** |
| 审计总线 | 先支持 **Kafka**（franz-go），后续框架化支持更多 broker |
| actor 模型 | **通用 principal 模型**，不直接镜像 Keycloak claims |
| 审计规则配置方式 | 采用 **all-in-proto + 注解 + 代码生成 + middleware** |
| broker / transport 演进方向 | 在 Servora 内部建设自有 `pkg` 生态，参考外部项目但不以其为核心依赖 |

---

## 3. 职责分工（不变）

### 3.1 Keycloak
负责用户认证、OIDC/OAuth2 标准流程、token 签发、JWKS/discovery、client/realm/role 管理。
不负责业务资源级授权、OpenFGA 关系建模、审计存储。

### 3.2 网关（Traefik）
负责统一入口、对接 Keycloak、验证 token、将 principal 注入上游请求头、粗粒度入口控制。
不负责细粒度授权判断、业务资源审计。

### 3.3 各业务服务
从 gateway header 构建 actor → 本地 `pkg/authz` → 直接调用 OpenFGA → 产出审计事件。

### 3.4 OpenFGA
关系模型存储、Check/ListObjects/tuple write/delete。

### 3.5 Audit Service（✅ Phase 2b 已建）
消费审计 topic → 校验反序列化 → 落库（ClickHouse） → 提供查询统计能力。

---

## 4. Phase 1 已完成：框架骨架

> 详细设计、spec 与实现索引见 `openspec/changes/archive/2026-03-20-framework-audit-skeleton/`。

| 交付物 | 关键决策 |
|--------|---------|
| pkg/logger v2 ⚡ | 暴力重构：`New(app)` / `For(l,"mod")` / `With(l,"mod")` / `Zap()` / `Sync()` |
| Actor v2 ⚡ | 扩展为完整身份模型（Subject/ClientID/Realm/Roles/Scopes/Attrs），新增 ServiceActor |
| IdentityFromHeader v2 | 8 种 gateway header → Actor v2，支持 WithHeaderMapping |
| audit.proto + annotations.proto | AuditEvent、4 typed detail、AuditRule method option |
| conf.proto 扩展 | Kafka（含 SASL）、ClickHouse、Audit 配置 |
| pkg/broker + kafka | **franz-go**（非 sarama）；Broker/Event/Subscriber/MiddlewareFunc 接口；参考 kratos-transport |
| pkg/audit 骨架 | Emitter → Recorder → Middleware；3 种 emitter（Noop/Log/Broker） |
| 基础设施 | Kafka KRaft + ClickHouse；IAM/sayhello 从工具链移除 |

### 实现约束收敛（Phase 1 + 2a 合并）

以下约束在 Phase 1–2a 实现过程中确立，后续阶段必须遵循：

1. **Optional-init 模式统一**：所有可选基础设施组件（Kafka、ClickHouse、OpenFGA）使用 `NewXxxOptional` 函数，nil 配置返回 nil 而非 panic，调用方 nil-check 后使用
2. **Proto 集中配置**：所有框架级配置（Kafka/ClickHouse/Audit）通过 `api/protos/servora/conf/v1/conf.proto` 统一管理，不做分散的 Go config struct
3. **Logger 桥接模式**：第三方库（franz-go kzap、GORM、Ent）通过 `logger.Zap()` 获取底层 `*zap.Logger`，不直接传递 Kratos `log.Logger`
4. **Module 命名规范**：`logger.For(l, "module")` 中 module 使用 `component/layer/service` 格式（如 `"clickhouse/data/audit"`、`"kafka/broker/pkg"`、`"core/data/iam"`），不带 `-service` 后缀。pkg 层用 `pkg` 作 service 段（如 `"kafka/broker/pkg"`），app 层用服务名（如 `"clickhouse/data/audit"`）
5. **broker 接口扩展点**：新增 broker 实现（NATS、RabbitMQ 等）只需实现 `broker.Broker` interface，不需修改 `pkg/broker` 核心
6. **OpenSpec 主 spec 格式**：必须包含 `## Purpose` section、`## Requirements`（非 `ADDED Requirements`）、每条 requirement 第一行含 SHALL/MUST、至少一个 `#### Scenario`
7. **Proto 包治理规范**：新增或迁移后的 proto 必须使用 `servora.*` package、目录需与 package 命名空间对齐、`go_package` 必须落到 `api/gen/go/servora/**`，对应主 spec 为 `openspec/specs/proto-package-governance/spec.md`
8. **pkg 框架包去特化原则**：`pkg/` 下的框架包不得包含任何业务特化逻辑（如硬编码 `"user:"` 前缀、硬编码业务 model 的 computed relations）。业务特定配置通过 functional option 由调用方注入
9. **ClientOption 模式**：`pkg/openfga.NewClient(cfg, opts...)` 接受 `ClientOption`（`WithAuditRecorder`、`WithComputedRelations`）；`NewClientOptional` 透传 opts。服务层通过 wrapper 函数注入特定 options（如 IAM 的 `NewOpenFGAClient`），再注册到 Wire ProviderSet
10. **core/public 分层模式**：涉及 cross-cutting concern（audit、metrics、tracing）的方法，拆为 unexported core 方法（纯操作）+ 导出 wrapper（组合 cross-cutting 逻辑）。后续新增 cross-cutting 只修改 wrapper，不碰 core
11. **Kafka 双 listener**：docker-compose 中 Kafka 配置 PLAINTEXT (9092, 容器间) + EXTERNAL (29092, 宿主机)，确保开发环境测试与容器间通信均可用
12. **Kafka topic 预创建**：`KAFKA_AUTO_CREATE_TOPICS_ENABLE: "true"` 已启用，但 franz-go 客户端首次发布时不等待 auto-create 完成，可能导致第一条消息失败（`UNKNOWN_TOPIC_OR_PARTITION`）。开发环境 topic 已持久化后无影响；生产环境应在服务启动前通过 admin API 或 init job 确保 topic 存在
13. **nil Timestamp 空值处理**：protobuf `*timestamppb.Timestamp` 在字段未设置时为 nil；调用 `.AsTime()` 返回 `time.Unix(0,0).UTC()`（非 Go zero value），`!t.IsZero()` 为 true 会产生意外 WHERE 条件。规范：凡接收 proto timestamp 参数并转 `time.Time` 的地方，必须先判断 nil
14. **Data 层结构统一**：每个微服务的 `internal/data/` 必须包含 `Data` struct + `NewData` 函数（持有底层连接、管 cleanup、跑 DDL）；各 repo 通过 `*Data` 访问连接（如 `d.ClickHouse()`、`d.Ent(ctx)`），不直接依赖裸连接。连接建立由独立函数完成（如 `NewClickHouseClient`、`NewDBClient`），`NewData` 负责组装
15. **分层依赖与接口返回**：`service` → `biz`（use case）→ `data`（repo），禁止跨层依赖。data 层构造函数直接返回 biz 层接口类型（如 `func NewAuditRepo(...) biz.AuditRepo`），Wire 自动解析，不使用 `wire.Bind`。数据接入管道（如 Kafka consumer）属于 data 层而非 biz 层；同 package 内组件（如 consumer → batch writer）直接依赖具体类型

---

## 5. 不保留中央 IAM/AuthZ 在线代理（不变）

核心理由保持不变：
1. `pkg/authz` 已具备通用执行能力，无需再套代理
2. OpenFGA 自身已是独立基础设施
3. 减少网络跳数与故障面
4. 授权决策本地执行更容易获取业务上下文

允许保留薄中心能力：OpenFGA model/store 管理、后台 tuple 管理、审计查询、运维控制台。

---

## 6. actor 模型（Phase 1 已实现）

actor 不直接等于 Keycloak claims。采用：

```text
Keycloak claims / gateway headers → adapter → actor.Actor
```

actor 字段：Type（user|service|anonymous|system）、ID、Subject、ClientID、Realm、DisplayName、Email、Roles、Scopes、Attrs。

`pkg/authz`、`pkg/audit`、业务服务只依赖 actor，不依赖 Keycloak 原始 claims 结构。

Keycloak 主集成方式：OIDC discovery、JWKS、token/introspection/userinfo、Admin REST（非 gRPC）。

---

## 7. 审计架构（Phase 1–2b 全链路已实现）

### 7.1 总体架构

```text
业务服务本地产生审计事件 → pkg/broker (Kafka) → Audit Service → ClickHouse → 查询 API
```

### 7.2 四类事件来源

| 事件类型 | 锚点 | 状态 | 说明 |
|----------|------|------|------|
| `authn.result` | `pkg/authn` / identity adapter | proto 已定义 | Phase 4 接入（随 Keycloak 改造） |
| `authz.decision` | `pkg/authz.Authz` middleware | ✅ Phase 2a 已接入 | `WithAuditRecorder` 直接注入，含 CacheHit |
| `tuple.changed` | `pkg/openfga` tuple write/delete | ✅ Phase 2a 已接入 | 方法内置自动 emit，core/public 分层 |
| `resource.mutation` | 业务服务 handler | proto 已定义 + middleware 骨架 | Phase 3 通过 annotation 自动化 |

### 7.3 all-in-proto 路线

```text
proto 注解 → protoc-gen-servora-audit → middleware 自动执行
```

结构：
- `api/protos/servora/audit/v1/audit.proto` — ✅ 已定义
- `api/protos/servora/audit/v1/annotations.proto` — ✅ 已定义（AuditRule + audit_rule method option）
- `cmd/protoc-gen-servora-audit` — Phase 3 实现
- `pkg/audit` runtime — ✅ 骨架已实现

---

## 8. 框架化演进方向

### 8.1 pkg 生态当前状态

| 包 | 状态 | 说明 |
|----|------|------|
| `pkg/actor` | ✅ v2 | 通用 principal 模型，4 种 actor type |
| `pkg/authn` | 🔄 待降级 | Phase 4 改造为身份适配层 |
| `pkg/authz` | ✅ 审计已接入 | `WithAuditRecorder` 直接注入，Check 后自动 emit `authz.decision` |
| `pkg/audit` | ✅ 主链已接入 | Recorder + LogEmitter/BrokerEmitter；e2e Kafka round-trip 已验证 |
| `pkg/broker` | ✅ 接口 + kafka 实现 | franz-go，kzap + kotel |
| `pkg/db/clickhouse` | ✅ 框架级封装 | `NewConnOptional`：proto conf → native driver，TLS/压缩/连接池 |
| `pkg/logger` | ✅ v2 | 暴力重构后的简洁 API |
| `pkg/openfga` | ✅ 框架化完成 | ClientOption 模式、API 去特化、core/public 分层、tuple audit emit |
| `pkg/transport` | ✅ 可用 | IdentityFromHeader v2 已升级 |

### 8.2 关于参考项目

| 项目 | 本地路径 | 参考内容 | 不参考内容 |
|------|---------|---------|-----------|
| kratos-transport | `/Users/horonlee/projects/go/kratos-transport` | broker 接口设计、Event/Subscriber/Handler 类型签名、option 组织、middleware 模式 | 整套外部抽象边界、直接作为依赖 |
| Kemate | `/Users/horonlee/projects/go/Kemate` | docker-compose 配置（Kafka KRaft）、optional-init 模式 | sarama 选型、Kafka Go 库代码 |

### 8.3 目录边界

- `pkg/transport`：请求/响应型能力（HTTP/gRPC/SSE/WebSocket），middleware，metadata 透传
- `pkg/broker`：消息型、事件型能力，broker interface，producer/consumer lifecycle
- `pkg/task` 或 `pkg/queue`（未来）：任务队列（Asynq 等），不强塞进 broker

---

## 9. 该删什么、留什么、换什么

### 9.1 已执行的变更

| 组件 | 操作 | 状态 |
|------|------|------|
| IAM/sayhello 工具链入口 | 从 Makefile MICROSERVICES/GO_WORKSPACE_MODULES 移除 | ✅ Phase 1 |
| IAM/sayhello 源代码 | 保留作为新服务参考模板，可独立编译 | ✅ 保留 |
| sayhello docker-compose.dev | 重新加入 docker-compose.dev.yaml 作为审计事件 E2E 发布者（port 10002），但不纳入 Makefile 工具链 | ✅ Phase 2b |
| `pkg/actor` | v2 破坏性升级 | ✅ Phase 1 |
| `pkg/logger` | 暴力重构 | ✅ Phase 1 |
| `pkg/transport/.../identity` | v2 多 header 支持 | ✅ Phase 1 |
| `pkg/openfga` | API 框架化 + ClientOption + core/public 分层 + audit emit | ✅ Phase 2a |
| `pkg/authz` | `WithAuditRecorder` + `authz.decision` emit + CachedCheck 适配 | ✅ Phase 2a |
| `pkg/audit` | 主链接入 + e2e 验证（LogEmitter + BrokerEmitter Kafka） | ✅ Phase 2a |
| `app/iam/service` | 适配 openfga 去特化 API + iamComputedRelations | ✅ Phase 2a |
| Kafka docker-compose | 新增 EXTERNAL listener (port 29092) 支持宿主机连接 | ✅ Phase 2a |
| `app/audit/service` | 新建审计微服务（Kafka consumer → ClickHouse → 查询 API） | ✅ Phase 2b |
| `pkg/db/clickhouse` | 新建框架级 ClickHouse 连接 helper，Optional-init 模式 | ✅ Phase 2b |
| sayhello docker-compose.dev | 重新加入作为审计 E2E 发布者（port 10002），不纳入 Makefile 工具链 | ✅ Phase 2b |

### 9.2 待执行（Phase 3+）

| 组件 | 操作 | 阶段 |
|------|------|------|
| `cmd/protoc-gen-servora-audit` | 新建：审计注解代码生成器 | Phase 3 |
| IAM issuer 能力（JWKS/OIDC/登录/注册） | 下线，认证交给 Keycloak | Phase 4 |
| Traefik → IAM /v1/auth/verify 链路 | 改为网关直接对接 Keycloak | Phase 4 |
| `pkg/authn` | 降级为身份适配层（gateway header mode + direct JWT mode） | Phase 4 |

---

## 10. 分阶段演进计划

### Phase 1：框架骨架 ✅ 已完成

> 交付物见 Section 4 表格。

### Phase 2a：审计 emit 接入（pkg 层） ✅ 已完成

> 详细设计、spec 与实现索引见 `openspec/changes/archive/2026-03-22-audit-emit-integration/`。

| 交付物 | 关键决策 |
|--------|---------|
| pkg/openfga API 框架化 ⚡ | `Check`/`ListObjects`/`CachedCheck` 参数从 `userID` 改为 `user`（完整 principal），移除 `"user:"` 硬编码；`parseTupleComponents` 通用化 |
| pkg/openfga ClientOption 模式 | `NewClient(cfg, opts...)` + `WithAuditRecorder` + `WithComputedRelations`；`NewClientOptional` 透传 |
| pkg/openfga core/public 分层 | `WriteTuples`/`DeleteTuples` 拆为 `writeTuplesCore`/`deleteTuplesCore` + public wrapper，成功后自动 emit `tuple.changed` |
| pkg/openfga CachedCheck 扩展 | 返回值 `(bool, error)` → `(bool, bool, error)`，新增 `cacheHit` |
| pkg/openfga 缓存层去特化 | `affectedRelations` 硬编码移除，改为 `Client.computedRelations`（通过 `WithComputedRelations` 注入）；`InvalidateForTuples` 改为 `Client` 方法 |
| pkg/authz audit 集成 | `WithAuditRecorder(r)` option + Check 后自动 emit `authz.decision`（含 CacheHit，allowed/denied/error 三种 decision） |
| app/iam/service 适配 | 全局适配 openfga 去特化 API；`NewOpenFGAClient` wrapper 注入 `iamComputedRelations` |
| Kafka EXTERNAL listener | docker-compose 新增 port 29092 供宿主机连接 |
| e2e 验证 | `pkg/audit/e2e_test.go`：LogEmitter JSON 输出 + BrokerEmitter Kafka round-trip（含 proto 反序列化） |

### Phase 2b：Audit Service + ClickHouse ✅ 已完成

> 详细设计、spec 与实现索引见 `openspec/changes/archive/2026-03-20-audit-service-clickhouse/`。

| 交付物 | 关键决策 |
|--------|---------|
| `app/audit/service` 微服务 | 严格分层（service→biz→data）；data 构造函数返回 biz 接口，不用 `wire.Bind`；consumer/batch-writer 归 data 层；proto 拆 `audit.proto` + `i_audit.proto`；hot-reload via `Dockerfile.air` |
| ClickHouse 存储 | 官方 native driver `clickhouse-go/v2`；`detail` 纯 JSON 列；DDL 内嵌 `CREATE TABLE IF NOT EXISTS`（启动时 idempotent） |
| `pkg/db/clickhouse` | 框架级连接 helper `NewConnOptional`（TLS/压缩/连接池），遵循 Optional-init 模式 |
| Kafka consumer | 复用 `pkg/broker.Subscribe`（不直接依赖 franz-go）；BatchWriter 按 `consumer_batch_size`/`consumer_flush_interval` 刷盘 |
| 查询 API | `ListAuditEvents`（cursor 分页 + 时间/类型/actor/service 筛选）、`CountAuditEvents`；gRPC + HTTP 转码 |
| 服务治理 | Consul 注册 + Traefik 路由（`/v1/audit`）+ OTel；端口 10000(HTTP)/10001(gRPC) |
| Data 层结构 | IAM 模式 `Data` struct：`NewClickHouseClient` 建连接 → `NewData` 持有 conn + 跑 DDL + cleanup → repo 依赖 `*Data` |
| E2E 验证 | sayhello（port 10002）作为发布者：Hello RPC → audit middleware → Kafka → audit service → ClickHouse → `GET /v1/audit/events` 可见 |
| ClickHouse schema | `PARTITION BY toDate(occurred_at)` · `ORDER BY (service, event_type, occurred_at, event_id)` · `TTL occurred_at + INTERVAL N DAY`（retention_days 默认 90） |
| conf.proto 扩展 | `App.Audit` 新增 `consumer_batch_size` / `consumer_flush_interval` / `retention_days`；`Data.ClickHouse` 新增 `tls` / `tls_skip_verify` / `compress` |

### Phase 3：all-in-proto 代码生成

**目标：** 审计走向声明式，减少手写 emit 逻辑。

**过渡模式（Phase 2b 已确立）：**
在 `protoc-gen-servora-audit` 实现前，审计规则在 gRPC server 初始化时手动注册：
- key 使用 proto 生成的 `_FullMethodName` 常量（如 `sayhellov1.SayHelloService_Hello_FullMethodName`），禁止字符串字面量
- 每个 RPC 按需手动配置 `audit.Rule{EventType, TargetType, RecordOnError}`

**核心任务：**
1. 实现 `cmd/protoc-gen-servora-audit`
2. 生成 operation → audit rule map（替代手动 WithRules 注册）
3. 生成字段提取 helper + detail builder
4. middleware 自动按 proto 规则执行审计
5. 集成到 `make api` 生成链路（`buf.audit.gen.yaml`）
6. 以 sayhello 为验证目标：生成产物对比现有手写 `WithRules`，确认等价后移除手写版

### Phase 4：Keycloak 接入

**目标：** 完成认证链路切换，下线自建 IAM issuer 能力。

**核心任务：**
1. 部署 Keycloak（docker-compose 新增 keycloak 服务）
2. 配置 Traefik 对接 Keycloak（ForwardAuth 或 OIDC middleware）
3. `pkg/authn` 降级重构：
   - Gateway identity mode（默认）：从 header 构造 actor
   - Direct JWT verification mode：极少数绕过网关的场景
4. 清理 IAM 中的 issuer/verify/JWKS/OIDC/登录注册能力
5. 前端对接 Keycloak 登录流程

### Phase 5：Servora 生态扩展

**目标：** 框架能力泛化，为对外发布做准备。

**方向：**
1. `pkg/broker` 补更多实现（NATS / RabbitMQ / Redis Streams）
2. 设计 `pkg/task` / `pkg/queue`（Asynq 等任务队列）
3. 统一框架级 observability、eventbus、identity、audit、authz 能力
4. 将 Servora 逐步沉淀为对外发布的微服务框架生态

---

## 11. 最终结论

本次设计的核心不是"替换一个认证服务"，而是为 Servora 确立一套长期有效的边界：

- 认证交给 **Keycloak**
- 网关负责 **统一认证与 principal 注入**
- 授权由 **各业务服务本地执行 `pkg/authz` + OpenFGA**
- 审计采用 **本地 emit + Kafka + 中心 Audit Service**
- actor 设计为 **通用 principal 模型**
- 审计与授权逐步走向 **all-in-proto + 注解 + 代码生成 + middleware**
- broker / transport / audit / authz / actor 构成 **Servora 的 pkg 框架生态**

Servora 未来围绕明确的基础设施边界、清晰的框架能力分层、通用的 proto 驱动与代码生成能力、面向微服务脚手架的长期 pkg 生态持续演进。
