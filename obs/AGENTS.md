# AGENTS.md - obs/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-05-21 -->

## 模块定位

`obs` 提供日志、OpenTelemetry tracing 与 Prometheus metrics 的运行时装配和适配层。

## 子目录职责

| 目录 | 职责 |
| --- | --- |
| `logger/` | 从 Bootstrap proto 构建 `*slog.Logger`，支持 stdout/file/OTel fanout，并由 bootstrap 绑定到 Kratos v3 默认 logger |
| `tracing/` | OTel trace provider 初始化与关闭 |
| `metrics/` | OTel metrics runtime、Prometheus `/metrics` handler、server/client request instruments |

## 边界约束

- TLS/CA 解析统一使用 `security/tls`，tracing 和 logger 复用该实现。
- logger 默认从 `corev1.Bootstrap.obs.log` 读取配置；调用方必须执行返回的 closer；Kratos v3 日志接入使用 `*slog.Logger`。
- tracing endpoint 为空时初始化返回 noop cleanup。
- metrics 默认从 `corev1.Bootstrap.obs.metrics` 读取配置；启用后使用 OTel `MeterProvider` 和私有 Prometheus registry，调用方必须执行返回的 cleanup。
- 业务自定义指标通过 `metrics.Metrics.Meter(name)` 创建 OTel instruments。

## 常见反模式

- 在 logger 中硬编码业务字段或服务名。
- 在 tracing/metrics 中发明独立配置结构绕过 Bootstrap proto。
- 在 metrics 中把服务名当作 OTel Meter name；服务名属于 Resource，Meter name 属于 instrumentation scope。
- 把原生 Prometheus 默认 registry 当作 Servora `/metrics` 的扩展点。
- 忘记关闭 logger/OTel 返回的 cleanup/closer。

## 测试

```bash
go test ./obs/...
```

修改 trace/metrics/logger 配置解析时，同时检查 `core/bootstrap` 的配置装配和 `api/protos/servora/core/v1` schema。
