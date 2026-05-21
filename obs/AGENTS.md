# AGENTS.md - obs/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-05-21 -->

## 模块定位

`obs` 承载框架可观测性能力：日志、OpenTelemetry metrics/tracing、审计事件。它提供运行时装配与适配层，不定义业务指标语义、业务审计模型或领域日志策略。

## 子目录职责

| 目录 | 职责 |
| --- | --- |
| `logger/` | 从 Bootstrap proto 构建 `*slog.Logger`，支持 stdout/file/OTel fanout 与 Kratos adapter |
| `logger/kratosv2/` | `slog` 到 Kratos logger 的适配 |
| `telemetry/` | OTel trace provider 与 Prometheus metrics 构造 |
| `audit/` | CloudEvents 审计事件、middleware 与后端 auditor |

## 边界约束

- `obs` 不承载认证、授权或 transport 业务逻辑。
- TLS/CA 解析使用 `security/tls`，不要在 telemetry/logger/audit 中复制证书加载逻辑。
- logger 默认从 `corev1.Bootstrap` 读取配置；调用方必须执行返回的 closer。
- tracing endpoint 为空时初始化返回 noop cleanup，不应强制报错。
- audit runtime 以 CloudEvents `Auditor.Emit` 为边界；authn/authz 失败/拒绝事件由安全包在配置 auditor 后直接发送。

## 常见反模式

- 在 logger 中硬编码业务字段或服务名。
- 在 telemetry 中发明独立配置结构绕过 Bootstrap proto。
- 在 audit 中反向 import `security/*` 实现包或业务资源模型。
- 忘记关闭 logger/OTel 返回的 cleanup/closer。

## 测试

```bash
go test ./obs/...
```

修改 trace/metrics/logger 配置解析时，同时检查 `core/bootstrap` 的配置装配和 `api/protos/servora/core/v1` schema。
