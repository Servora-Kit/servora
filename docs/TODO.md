# Servora 优化待办清单

> 创建于 2026-04-25。本文件记录 servora 框架的已知缺陷与优化点，按优先级分段。
> 完成的条目移至文末「已完成」段，并保留简要结论 + PR 链接。
> PR / commit message 中可直接引用条目编号，如 `fixes TODO.md P0-1`。

---

## P0 — 关键缺陷（影响功能完整性，建议尽快闭环）

### [P0-1] Audit middleware 事件分派不完整（仅 stub）

- **现状**：`obs/audit/middleware.go:62-63` 自述为 "skeleton, full implementation follows in phase 2"。switch 中只有 `EventTypeResourceMutation` 一条路径触发 `RecordResourceMutation`（L116），其余 3 类事件运行时压根不会被发送。
- **影响**：`AuthzDecision` / `TupleChange` / `AuthnResult` 三类审计事件全部丢失，proto 注解 → middleware → emitter 闭环未通。
- **建议**：
  1. 在 middleware 中补全其余事件类型分派
  2. 单元测试覆盖 4 种 EventType
  3. E2E 测试验证「proto 注解 → 中间件 → Broker/Log emitter」端到端落库

### [P0-2] Authz 决策未自动桥接到 Audit Recorder

- **现状**：`security/authz/authz.go:199-201` 提供了 `WithDecisionLogger` 回调，但需要业务层手动转发到 `audit.Recorder`，极易遗漏。
- **建议**：
  1. 在 middleware 栈中默认注册 audit bridge，自动把每次授权决策记录为 `AUDIT_EVENT_TYPE_AUTHZ_DECISION`
  2. 提供 `audit.NewAuthzBridge(recorder)` 便利函数
  3. 文档说明如何关闭（如出于性能考虑）

### [P0-3] Authn 失败事件未审计

- **现状**：`security/authn/authn.go` 100 行的实现，认证失败仅返回 error，没有审计钩子。
- **建议**：
  1. 增加 `WithAuditRecorder` 选项
  2. 失败时记录认证方式（JWT/API Key/Session）、原因、源 IP
  3. 与 P0-1、P0-2 同一批次完成，统一三件套闭环

### [P0-4] 框架自身缺少 proto 注解端到端示例

- **现状**：注解定义（`api/protos/servora/{audit,authz}/v1/`）和 protoc 插件（`cmd/protoc-gen-servora-{audit,authz,mapper}`）都齐全，但 `api/protos/` 中零处使用。生成的 `*_rules.gen.go` 文件数为 0。
- **影响**：使用者没有可参考的最小可运行样例，复制粘贴成本高。
- **建议**：在 `api/protos/` 新增 `example/v1/greeting.proto`，标注 audit + authz 注解，作为 lighthouse example。`buf generate` 后应产出对应 `.gen.go`。

---

## P1 — 安全与契约规范化

### [P1-1] 移除初始化 panic，改 error 返回

- **现状**：7 处生产代码在初始化失败时直接 panic：
  - `transport/server/grpc/server.go:69` — resolve grpc registry endpoint
  - `transport/server/http/server.go:71` — HTTP 同上
  - `transport/client/grpc/dialer.go:71` — client config index
  - `platform/config/etcd.go:72` — etcd client 创建失败
  - 等共 7 处
- **建议**：
  1. 关键路径返回 error，由 `main()` 决定退出策略
  2. 保留 `MustXxx()` 版本仅供本地/测试场景
  3. 每个改动点补单元测试

### [P1-2] Dialer 移除 `ctx == nil` 兜底

- **现状**：`transport/client/grpc/dialer.go:82-85` 与 `transport/client/http/dialer.go:81-84` 都在 `ctx == nil` 时回退到 `context.Background()`，违反 Go context 规范，会吞掉超时与取消信号。
- **建议**：移除 nil 检查，要求调用方传有效 context，文档明确契约。

---

## P2 — 可观测性一体化

### [P2-1] Log/Trace 一体化

经实证（curl + 日志对照）：Kratos `tracing.Server()` + `logging.Server()` 链路已让 **access log** 自动带 trace_id；缺口集中在 **业务 helper 手记日志** 与 **DB 层日志**。原合并条目按修复路径拆为 4 条独立子任务。

#### [P2-1a] Ent driver 加 trace_id / span_id

- **现状**：`obs/logging/ent_log.go` 的 `EntLogFuncFrom` 返回 `func(...any)` 无 ctx 入口；`infra/db/ent/driver.go` 也未做 dialect.Driver wrapper。所有 Ent SQL 调用日志 trace_id 必空。
- **方案**：新增 `tracingDriver` 实现 `dialect.Driver` 接口，每次 Query/Exec/Tx 抽 ctx 里的 SpanContext 写结构化日志；通过 `NewDriverWithTracing` 显式启用，零破坏。
- **Plan**：[`superpowers/plans/2026-04-25-ent-trace-correlation.md`](superpowers/plans/2026-04-25-ent-trace-correlation.md) — 9 个 Task

#### [P2-1b] 业务侧日志 ctx 绑定规范

- **现状**：servora-platform/audit data 层 10 处 `r.log.Warnf(...)` 全部裸调用，trace_id 必空。框架 godoc 也没说清 `*log.Helper.WithContext(ctx)` 是激活 valuer 的唯一姿势。
- **方案**：framework 侧加 `ExampleHelper_WithContext` 示例 + 修订 godoc + `AGENTS.md` 增设「日志规范」章节；业务侧迁移 audit/data 层 10 处调用 + 清理 servora-example 实验残留。
- **Plan**：[`superpowers/plans/2026-04-25-business-log-ctx-binding.md`](superpowers/plans/2026-04-25-business-log-ctx-binding.md) — 8 个 Task

#### [P2-1c] GormLogger 加 trace_id / span_id（辅助跟随，未写 plan）

- **现状**：`obs/logging/gorm_log.go` 4 个方法（Info/Warn/Error/Trace）签名都接收 `ctx context.Context`，但方法体内完全不用 ctx。
- **优先级与定位**：servora 自身与 servora-platform 当前都不用 GORM；待真有业务仓库使用时再做，或在 P2-1a 完工后顺手补。约 30 分钟。
- **方案**：4 个方法体内加 `if sc := trace.SpanContextFromContext(ctx); sc.IsValid() { ... }` 注入 zap field。

#### [P2-1d] valuer invalid 时省略字段（视觉小优化，未写 plan）

- **现状**：`platform/bootstrap/bootstrap.go:103-104` 把 `tracing.TraceID()` / `tracing.SpanID()` 注册为全局 valuer。bootstrap 阶段无 active span，valuer 输出空字符串，导致启动日志出现 `"trace_id":"","span_id":""` 视觉污染。
- **优先级与定位**：5 分钟改完。
- **方案**：包一层 valuer，invalid SpanContext 时返回 `nil`，让 zap encoder 直接省略字段。需评估是否影响日志 schema 稳定性（部分 ELK pipeline 可能依赖字段必存在）。

### [P2-2] `/metrics` 与 `/healthz` 拆到独立 admin 端口

- **现状**：`transport/server/http/server.go:84-91` 将 `/metrics` 和 `/healthz` 直接挂在业务 HTTP server，继承全部业务中间件（auth、audit、CORS），既增加无谓开销也带来潜在安全风险（暴露内部指标到公网）。
- **建议**：
  1. 新增 `internal/admin` server，默认监听 `127.0.0.1:9090`
  2. HTTP server 增加 `WithAdminAddr(addr string)` 选项
  3. 优雅关闭时业务 server 与 admin server 都需 Stop

### [P2-3] 配置热更新加 metrics + validation hook

- **现状**：`platform/config/etcd.go:183-194` 的 `watcher.Next()` 每次返回 `source.Load()`（全量）而非增量，且无可观测性。坏配置会被静默应用。
- **建议**：
  1. Watcher 暴露指标：`config_reload_total` / `config_reload_duration_seconds` / `config_validation_failed_total`
  2. 提供 `WithValidator(func(*conf.Bootstrap) error)` 选项，校验失败保留旧配置
  3. 每次 reload 打 debug 日志含 revision/timestamp，便于追踪漂移

---

## P3 — 测试深度与开发体验

### [P3-1] Transport 加 e2e 与 goroutine leak 检测

- **现状**：40 个测试中仅 12 个涉及 transport，且都是构造层单元测试，未启动真实 HTTP/gRPC server，未测试 Dialer/Watcher 的资源清理。
- **建议**：
  1. 新增 e2e 测试：启动真实 server + mock 注册表，验证端点解析、重试、优雅关闭
  2. 接入 `go.uber.org/goleak` 检查 Dialer/Watcher 协程泄漏
  3. 新增 `BenchmarkDial` 衡量连接建立成本

### [P3-2] 各模块补 doc.go 包级文档

- **现状**：仅 13 个文件含包级注释，多数子包无 `doc.go`。
- **建议**：为 `core/` `transport/` `security/` `obs/` `infra/` `platform/` 每个子包补 `doc.go`，2-5 行说明用途与典型用法。

---

## 已完成

（暂无）

---

## 维护说明

- 完成一项后将该条目移至「已完成」段，保留 1-2 行结论 + 关联 PR/commit。
- 新增项请标注优先级（P0/P1/P2/P3）+ 文件路径证据 + 简短建议。
- 优先级判断：P0 = 功能未闭环；P1 = 契约/安全规范；P2 = 可观测性/运维；P3 = 测试/文档。
- 与 `docs/superpowers/plans/` 区别：本文件是**持续维护**的待办清单；`docs/superpowers/plans/` 是**一次性**专题 plan/RFC（带日期，由 superpowers writing-plans skill 产出，含 TDD checkbox 步骤）。条目升级为正式专题时迁移到 `docs/superpowers/plans/YYYY-MM-DD-<topic>.md`，并在条目下追加 `**Plan**:` 相对链接。
