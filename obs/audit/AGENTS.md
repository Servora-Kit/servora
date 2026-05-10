# AGENTS.md - obs/audit/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-22 | Updated: 2026-05-07 -->

## 模块目的

提供审计事件运行时能力：定义 `Recorder` 与 `Emitter` 抽象、`Collector` middleware、push-ctx detail helpers，把跨层产生的认证 / 授权 / 资源 / OpenFGA tuple 事件统一写成 `auditpb.AuditEvent` 投递给后端。

## 当前文件

- `event.go`：`EventID` 等内部 envelope 辅助
- `enums.go`：纯 Go 枚举别名（`EventType` / `AuthzDecision` / `TupleMutationType` / `ResourceMutationType`）—— 仅作为字符串可读性 wrapper，不是 schema 双轨
- `emitter.go`：`Emitter` 接口
- `recorder.go`：`Recorder` + 高阶 `RecordAuthzDecision` / `RecordTupleChange` / `RecordResourceMutation` / `RecordAuthnResult`，以及低阶 `Emit`
- `broker_emitter.go` / `log_emitter.go` / `noop_emitter.go`：emitter 后端实现
- `middleware.go`：proto-annotation 驱动的 `RESOURCE_MUTATION` 中间件骨架
- `context.go`：push-ctx detail holder（`InstallHolder`）+ 4 个 ctx helper（`WithAuthnResult` / `AuthnResultFrom` / `WithAuthzResult` / `AuthzResultFrom`）
- `collector.go`：`Collector` 中间件 + `WithSpanEvents` Option
- `config.go`：装配辅助
- `proto.go`：(已并入 `enums.go` / `recorder.go`，保留为占位，proto 转换逻辑直接使用 `auditpb.*`)

## 当前实现事实

- `Emitter` 暴露 `Emit(ctx, *auditpb.AuditEvent)` 与 `Close()` 生命周期接口
- emit 失败不应影响主业务流程；审计是旁路能力
- **schema 单一来源是 proto**（`api/gen/go/servora/audit/v1`，BSR `buf.build/servora/servora`）。v0.4.4 起 runtime detail 结构体（`AuthnDetail` / `AuthzDetail` / `AuditEvent`）已删除，所有 API 直接使用 `auditpb.*`，无手写 mapper（G1 schema-as-SoT）
- `Recorder.AuthnObserver()` / `Recorder.AuthzObserver()` callback 桥接已**移除**（v0.4.4 break）。security middleware 不再走 callback，改为 push-ctx
- 包内同时提供 broker / log / noop 三种 emitter 适配不同部署形态

### AuditActor 字段填充责任划分

`auditpb.AuditActor` proto schema **保持 7 个字段不变**：`id` / `type` / `display_name` / `email` / `subject` / `client_id` / `realm`。但 `core/actor.Actor` 已被精简为「三件套」（`ID()` / `Type()` / `DisplayName()`），不再持有 OIDC 元数据。因此：

- **框架 Recorder（本包 `recorder.go`）**：只负责填 `Id` / `Type` / `DisplayName` 三个字段——这是任意身份引擎都能稳定提供的最小集合
- **业务侧 Recorder Wrapper**：若业务关心 `Email` / `Subject` / `ClientId` / `Realm`，需在自己的 wrapper 里从业务专属的 ctx 通道读出（典型如 `iam.UserInfoFrom(ctx)`），再二次设值后 `Emit`。框架不假设这些字段存在，也不替业务从 `core/actor` 反向「补齐」

该划分让框架对身份协议（JWT / API Key / OIDC / 自研）保持中立，避免 `core/actor` 重新长出协议特定字段。

## Push-ctx pipeline

`security/{authn,authz}` middleware 通过 ctx 把 `*auditpb.AuthnDetail` / `*auditpb.AuthzDetail` 推给 `obs/audit`；`Collector` middleware 在请求末端单点 emit。security 包对 `obs/audit` 的依赖只剩中立 schema 包 `auditpb`（不再 import emit 实现）。

API surface：

```go
// ctx helpers（被观察侧——security middleware——调用）
func WithAuthnResult(ctx context.Context, d *auditpb.AuthnDetail) context.Context
func AuthnResultFrom(ctx context.Context) (*auditpb.AuthnDetail, bool)
func WithAuthzResult(ctx context.Context, d *auditpb.AuthzDetail) context.Context
func AuthzResultFrom(ctx context.Context) (*auditpb.AuthzDetail, bool)

// 末端中间件（业务调用方装配）
func Collector(rec *Recorder, opts ...CollectorOption) middleware.Middleware
func WithSpanEvents(enabled bool) CollectorOption  // 默认 true
```

实现细节：`InstallHolder(ctx)` 在 `Collector` 入口装入一个 per-request 可变 holder。Go `context.WithValue` 不可变——若 inner middleware `context.WithValue(ctx, ...)` 写入新值，outer 在 LIFO 后置阶段拿到的是 ORIGINAL ctx，看不到子写。改为 holder（指针）+ 沿原 ctx 传递，inner mutate 同一 holder，outer 在 post-phase 取到最新写入。`InstallHolder` 是 export 但**业务侧通常不直接调用**——`Collector` 自动调用一次；它存在主要是为单元测试或非 Kratos 场景手工组装。

每次 `WithAuthnResult` / `WithAuthzResult` 写入会调用 `trace.SpanFromContext(ctx).AddEvent(...)`，事件名 `audit.authn.recorded` / `audit.authz.recorded`；`Collector` 自身在 emit 后再发 `audit.collected`（可经 `WithSpanEvents(false)` 关闭）。OTel 未配 / 当前 span 未采样时 AddEvent 是 SDK noop，零成本。

写 ctx 与 emit 分离的好处：authn/authz middleware 无需 import `Recorder`，未来加 audit 维度（如 client_ip、policy_version）只改 collector 一处，不动 security 包。

## Mounting位置（CRITICAL）

`Collector` **必须**挂在 authn / authz middleware 的**外层**（outer，slice 中位置在前）。Kratos middleware Chain 按 slice 顺序包装：列在前面的中间件 wrap 后面的，failure 时 inner 短路 return 跳过更内层但 outer 的 LIFO post-phase 仍执行——这是 authn 失败路径仍能 emit `AUTHN_RESULT` 事件的关键。

推荐链顺序：

```
recovery → tracing → logging → audit.Collector → authn → authz → handler
                               ^^^^^^^^^^^^^^^
                               必须 OUTER to authn/authz；放 tracing 后让 trace_id 进入 AuditEvent
```

错挂为 inner（如 `Chain(authn, authz, audit.Collector)`）的后果：

- authn 失败短路时 Collector 永不执行，`AUTHN_RESULT` 事件**静默丢失**
- holder 也未安装，inner 的 `WithAuthnResult` 写入静默 drop（但不 panic、不 error）
- 这是文档强制约束但**无运行时校验**——Kratos middleware 是匿名 closure 无法反射顺序。e2e 测试覆盖正确挂载（参 `e2e_test.go`）

## Audit event sources topology

`auditpb.AuditEventType` 有 4 种，分别由不同模块产生：

| EventType            | 源头                                                                       | 路径                                                                |
| -------------------- | -------------------------------------------------------------------------- | ------------------------------------------------------------------- |
| `AUTHN_RESULT`       | `security/authn` middleware 写 ctx                                         | `audit.WithAuthnResult` → holder → `Collector` 末端 emit            |
| `AUTHZ_DECISION`     | `security/authz` middleware 写 ctx                                         | `audit.WithAuthzResult` → holder → `Collector` 末端 emit            |
| `RESOURCE_MUTATION`  | `obs/audit/middleware.go`（proto 注解 `servora.audit.v1.audit_rule` 驱动）| 业务 RPC handler 完成后由 `Audit` middleware 直接 emit              |
| `TUPLE_CHANGED`      | `infra/openfga` 写路径                                                     | tuple 写入成功后直接 `Recorder.RecordTupleChange` emit               |

设计意图：每个 EventType 的 `Result` 反映该层自己的结果。handler 业务错误属 RPC 层信息，由 `RESOURCE_MUTATION` 独立记录，不污染 authn/authz 决策记录——consumer 可按 EventType 维度区分「authn 失败」与「authn ok 但业务失败」。

## 边界约束

- 本包负责"记录审计事件"，不负责认证 / 授权 / 风控 / 业务补偿
- 不在这里强行定义具体业务事件枚举；业务语义由调用方决定
- 不把审计失败升级为会中断请求的致命错误
- 不在 `obs/audit` 主包反向 import `security/{authn,authz}`（v0.4.4 起 hub 模式已替换为 push-ctx，反向 import 不再需要）

## 常见反模式

- 审计发送失败后直接返回 5xx 或 panic
- 在 `obs/audit` 包中塞入具体业务资源模型与领域判断
- 把 `Collector` 挂在 authn/authz 内层（失败路径丢事件）
- 在 security middleware 中直接 import `Recorder` 或 emitter（破坏单点 emission）
- 让 handler 业务错误污染 `AUTHN_RESULT.Result.ErrorMessage`

## 测试与使用

```bash
go test ./obs/audit/...
```

业务调用方装配示例：

```go
import (
    "github.com/Servora-Kit/servora/security/authn"
    "github.com/Servora-Kit/servora/security/authz"
    "github.com/Servora-Kit/servora/obs/audit"
)

recorder := audit.NewRecorder(emitter, "iam")
mw = []middleware.Middleware{
    recovery.Recovery(),
    tracing.Server(),
    logging.Server(l),
    audit.Collector(recorder),                 // ★ 必须 OUTER to authn/authz
    authn.Server(jwtAuth),
    authz.Server(fgaAuth, authz.WithRulesFunc(iampb.AuthzRules)),
}
```

## 维护提示

- 若新增 emitter 类型，优先保持 `Emitter` 接口最小稳定，不要把后端细节泄漏到调用方
- 若 proto schema（`api/protos/servora/audit/v1`）变更，执行 `make gen` 后 `Recorder` / `Collector` 的实现可能需同步更新；schema 是单一来源
- Kafka topic 命名：emitter 实现侧自定义；broker emitter 默认按服务/事件类型分 topic（具体见 `broker_emitter.go`）
- 若新增 EventType，需要在 proto 加 enum 值 + 在 collector 或对应源头模块加 build/emit 路径，并在本文件 topology 表格补一行
