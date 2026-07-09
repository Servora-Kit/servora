# AGENTS.md - obs/audit/

<!-- Parent: ../AGENTS.md -->
<!-- Updated: 2026-07-09 -->

## 模块定位

`obs/audit` 提供 CloudEvents 审计事件运行时能力：`Auditor` 抽象、CloudEvents 构造 helper、Kratos middleware、multi/noop/stdout/log/kafka 后端。

核心契约：

```go
type Auditor interface {
    Emit(ctx context.Context, event cloudevents.Event) error
}
```

本包不负责认证、授权、风控、补偿或业务资源模型；安全中间件只依赖 `Auditor` 抽象，不依赖具体后端。

## 事件来源

| 来源 | 语义 |
| --- | --- |
| `audit.Middleware` | 根据生成的 audit rules，在 handler 返回后构造并发送通用 RPC audit event |
| `security/authn` | 配置 `WithAuditor` 后，失败路径发 `servora.authn.failure.v1`，成功路径发 `servora.authn.success.v1` |
| `security/authz` | 配置 `WithAuditor` 后，allowed/denied/error 三条路径分别发 `servora.authz.allowed.v1`、`servora.authz.denied.v1`、`servora.authz.error.v1` |
| `security/authz/openfga` | tuple 写入成功后通过 `Client.auditor`（`audit.Auditor`）直接 emit `servora.authz.openfga.tuple_mutation.v1` |

每种事件只表达本层结果。handler 业务错误属于 audit rule 事件语义，不要污染 authn/authz 决策事件。

## Middleware 语义

```go
func Middleware(auditor Auditor, opts ...Option) middleware.Middleware
func WithRulesFuncs(fns ...func() map[string]*auditv1.AuditRule) Option
```

`CompiledRule` 已删除，rule 类型直接使用生成代码 `*auditv1.AuditRule`。`MiddlewareOption` 已重命名为 `Option`，`middlewareConfig` 已重命名为 `serverConfig`。`WithSubjectFunc` 与 `WithAuthTypeFunc` 已删除。

`Middleware` 构造期合并 rules，运行期先调用 handler，再构造 `servora.audit.rpc.v1` event，`subject` 设为 transport operation。handler 返回错误时写入私有 `errormessage` extension。emit 失败只写日志，不中断业务响应。

`NewEvent()` 从 `app.FromContext(ctx)` 读取应用名并设置 source 为 `"//appname"`，无法取到时回退为 `"//unknown"`。source 不再来自 transport operation。`NewEvent()` 不再注入 `severitytext` 或 `recordedtime`；若 span valid 且 sampled，则自动注入 OTel `traceparent`/`tracestate`。`WithSeverity` EventOption 已删除。

`Multi()` 函数已迁移至 `obs/audit/multi` 子包，使用方式改为 `multi.New(...)`；子包同时实现了 `Close()` 与 `Flush()` 传播。

推荐 server middleware 顺序：

```text
recovery -> tracing -> logging -> ratelimit -> validate -> metrics -> audit.Middleware -> authn -> authz -> handler
```

## CloudEvents 约束

CloudEvents extensions 由事件生产者或后端按职责写入：`NewEvent()` 只在 sampled span 存在时补 `traceparent`/`tracestate`，RPC middleware 只补 `errormessage`，authz 事件补 `authid`，Kafka backend 私有使用 `partitionkey`。`extensions.go` 已删除，不再暴露公开 `Ext*` 常量。

新增审计字段优先进入领域 typed data；需要平台索引或路由的少量元数据才使用 CloudEvents extension。不要重新引入旧版 runtime detail struct 双轨。

## 后端边界

- `noop/`：禁用或测试。
- `stdout/`：本地开发 JSON 输出。
- `log/`：本地/demo 的人类可读结构化 slog 输出。
- `kafka/`：基于 franz-go 的 CloudEvents Kafka 发送后端和轻量 binding adapter；Kafka client 接线复用 `contrib/kafka` 或调用方提供的 `*kgo.Client`。
- `multi/`：fanout 多个 auditor；单个后端失败不应改变业务返回。

## 常见反模式

- 在 audit 中反向 import `security/authn` 或 `security/authz` 实现包。
- 因审计发送失败中断主业务流程。
- 让 authn/authz 写旧式 ctx detail，再由外层中间件二次转发。
- 在本包硬编码业务 resource schema。

## 测试

```bash
go test ./obs/audit/...
```

重点覆盖：rule merge、handler 后 emit、错误 extension、emit 失败不阻断、multi/noop/stdout/log/kafka 后端、CloudEvents attribute helpers。
