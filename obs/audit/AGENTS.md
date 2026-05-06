# AGENTS.md - obs/audit/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-22 | Updated: 2026-03-22 -->

## 模块目的

提供审计事件运行时能力，围绕 `Event`、`Emitter`、`Recorder` 与 middleware 骨架组织统一的审计记录链路。

## 当前文件

- `event.go`：审计事件模型
- `emitter.go`：`Emitter` 接口定义
- `recorder.go`：记录器抽象
- `broker_emitter.go`：基于消息代理的 emitter
- `log_emitter.go`：基于日志输出的 emitter
- `noop_emitter.go`：空实现 emitter
- `middleware.go`：Kratos middleware 骨架
- `observers.go`：`Recorder.AuthnObserver()` / `Recorder.AuthzObserver()` 桥接器（本包**唯一**反向 import `security/{authn,authz}` 的文件）
- `proto.go`：proto 相关转换辅助
- `config.go`：配置装配辅助

## 当前实现事实

- `Emitter` 暴露 `Emit(ctx, event)` 与 `Close()` 生命周期接口
- emit 失败不应影响主业务流程，审计属于旁路能力而非交易主路径
- 当前 `middleware.go` 更偏骨架与占位，完整审计编排通常仍需业务侧补充上下文
- 包内同时提供 broker / log / noop 多种 emitter，以适配不同部署形态
- `Recorder.AuthnObserver()` / `Recorder.AuthzObserver()` 是给 `security/{authn,authz}` middleware 用的 callback 桥接器，业务侧通过 `authn.WithObserver(recorder.AuthnObserver())` / `authz.WithObserver(recorder.AuthzObserver())` 接线即可

## Observer 桥接（与 security/{authn,authz} 的连接点）

`Recorder.AuthnObserver()` 与 `Recorder.AuthzObserver()` 返回 `security/authn.Server` / `security/authz.Server` 中 `WithObserver(fn)` Option 期望的回调签名，把 middleware 的事件流接入审计管道：

```go
import (
    "github.com/Servora-Kit/servora/security/authn"
    "github.com/Servora-Kit/servora/security/authz"
    "github.com/Servora-Kit/servora/obs/audit"
)

recorder := audit.NewRecorder(emitter, ...)

authnMw := authn.Server(authenticator,
    authn.WithObserver(recorder.AuthnObserver()),
)
authzMw := authz.Server(authorizer,
    authz.WithRulesFunc(rules),
    authz.WithObserver(recorder.AuthzObserver()),
)
```

实现位置：两个方法均定义在 `obs/audit/observers.go`，是 `obs/audit` 主包**唯一**反向 import `security/authn` 与 `security/authz` 的文件。其他 `.go` 文件不得直接 import `security/{authn,authz}`。

### 依赖方向（hub 模式）

设计上的依赖方向是单向的：

```
obs/audit  →  security/authn
obs/audit  →  security/authz
```

`security/{authn,authz}` 主包对 `obs/audit` **零依赖**，只暴露 `WithObserver(fn)` 接口与 `AuthnDetail` / `DecisionDetail` 数据结构；`obs/audit` 反向观察它们。这与 tingo 等成熟脚手架的 hub 模式一致——audit 作为 observer 知道 security，security 不知道 audit 存在。

> 不要在 `security/authn` 或 `security/authz` 主包里 import `obs/audit`；如果未来某种新的桥接需要触达 security 之外的领域，桥接文件依然应该放在被观察侧之外（即 `obs/audit/` 或更上层的装配层），保持「observer 反向观察」的方向不变。

### nil-safe 行为

两个方法在 nil `*Recorder` 上调用都是合法的，返回 no-op 闭包：

```go
var r *audit.Recorder // nil
authn.Server(a, authn.WithObserver(r.AuthnObserver())) // OK，回调进 no-op
authz.Server(z, authz.WithObserver(r.AuthzObserver())) // OK，回调进 no-op
```

业务侧因此可以无条件接线 observer，不必先 nil-check recorder。

### Authn 字段映射

`security/authn.AuthnDetail` → `obs/audit.AuthnDetail`：

| 来源（authn）  | 去向（audit）   | 备注                               |
| -------------- | --------------- | ---------------------------------- |
| `Method`       | `Method`        | 原样转发（`"jwt"` / `"mtls"` 等）  |
| `Allowed`      | `Success`       | 布尔直接映射                       |
| `Err.Error()`  | `FailureReason` | `Err == nil` 时为 `""`             |
| `Subject`      | audit Actor     | 直接作为 `RecordAuthnResult` 的入参 |

`Operation` 由 audit 侧从请求的 transport context 解析（`transport.FromServerContext`）；context 无 transport 时为 `""`。

### Authz 三态映射

`security/authz.DecisionDetail` → `obs/audit.AuthzDetail.Decision`，按以下优先级判定（顺序重要）：

| 条件                                       | 映射为                |
| ------------------------------------------ | --------------------- |
| `d.Err != nil`                             | `AuthzDecisionError`  |
| `d.Err == nil && d.Allowed`                | `AuthzDecisionAllowed` |
| `d.Err == nil && !d.Allowed`               | `AuthzDecisionDenied` |

错误优先于 deny：授权后端故障（超时、网络错误、内部异常）与策略拒绝在告警 / SLO / 风控信号上含义不同，下游消费方必须能区分两者，不能合并成单一 `denied`。

audit Actor 由 `actor.FromContext(ctx)` 提取（沿用旧 bridge.go 行为）；context 中无 actor 时回退到 `actor.NewAnonymousActor()`。

## 边界约束

- 本包负责“记录审计事件”，不负责认证、授权、风控或业务补偿
- 不在这里强行定义具体业务事件枚举；业务语义由调用方决定
- 不把审计失败升级为会中断请求的致命错误

## 常见反模式

- 审计发送失败后直接返回 5xx 或 `panic`
- 在 audit 包中塞入具体业务资源模型与领域判断
- 把 middleware 当作唯一入口，忽略 recorder / emitter 的独立可组合性

## 测试与使用

```bash
go test ./obs/audit/...
```

## 维护提示

- 若新增 emitter 类型，优先保持 `Emitter` 接口最小稳定，不要把后端细节泄漏到调用方
- 若补全 middleware，请保持“失败不影响主流程”的默认原则
