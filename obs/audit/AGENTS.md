# AGENTS.md - obs/audit/

<!-- Parent: ../AGENTS.md -->
<!-- Updated: 2026-05-21 -->

## 模块定位

`obs/audit` 提供 CloudEvents 审计事件运行时能力：`Auditor` 抽象、CloudEvents extension helpers、Kratos middleware、multi/noop/stdout/kafka 后端。

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
| `audit.Middleware` | 根据生成的 audit rules，在 handler 返回后构造并发送 resource/business event |
| `security/authn` | 配置 `WithAuditOnFailure` 后，在认证失败时直接发送 CloudEvents |
| `security/authz` | 配置 `WithAuditOnDeny` 后，在 deny/check error 时直接发送 CloudEvents |
| `security/authz/openfga` | tuple 写入成功后可直接调用 auditor |

每种事件只表达本层结果。handler 业务错误属于 audit rule 事件语义，不要污染 authn/authz 决策事件。

## Middleware 语义

```go
type CompiledRule struct {
    Mode       int32
    EventType  string
    Severity   string
    BuildEvent func(ctx context.Context, req, resp any, err error) cloudevents.Event
}

func Middleware(auditor Auditor, opts ...MiddlewareOption) middleware.Middleware
func WithSubjectFunc(fn func(context.Context) (string, bool)) MiddlewareOption
func WithAuthTypeFunc(fn func(context.Context) (string, bool)) MiddlewareOption
func WithRulesFuncs(fns ...func() map[string]*CompiledRule) MiddlewareOption
```

`Middleware` 构造期合并 rules，运行期先调用 handler，再构造 event、补 `authid`/`authtype` extensions，最后调用 `auditor.Emit`。emit 失败只写日志，不中断业务响应。

推荐 server middleware 顺序：

```text
recovery -> tracing -> logging -> ratelimit -> validate -> metrics -> audit.Middleware -> authn -> authz -> handler
```

## CloudEvents 约束

Servora audit extensions 定义在 `extensions.go`：`authid`、`authtype`、`traceparent`、`tracestate`、`severitytext`、`recordedtime`、`partitionkey`、`errormessage`。

新增审计字段优先走 CloudEvents extension 或 proto/generated rule，再接入 builder；不要重新引入旧版 runtime detail struct 双轨。

## 后端边界

- `noop/`：禁用或测试。
- `stdout/`：本地开发 JSON 输出。
- `kafka/`：CloudEvents Kafka 发送 stub/适配，不等同于 `infra/broker/kafka`。
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

重点覆盖：rule merge、handler 后 emit、auth metadata extensions、emit 失败不阻断、multi/noop/stdout/kafka 后端、CloudEvents attribute helpers。
