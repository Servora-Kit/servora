# servora

Shared protobuf definitions for the [Servora](https://github.com/Servora-Kit/servora) microservice framework.

## Layout

Proto files are organized in these top-level groups under `servora/`:

| Group | Purpose | Members |
|-------|---------|---------|
| Annotations (flat) | Extension annotations consumed by `protoc-gen-servora-*` plugins. Each namespace holds a single `annotations.proto`. | `audit/v1` / `conf/v1` |
| `redact/v3/` | Field-level log-redaction annotations consumed by `protoc-gen-redact`. | `redact.proto` |
| `core/v1/` | Framework startup bootstrap model. | `bootstrap.proto` (Bootstrap / App / Server / Data / Registry / Source / Observability) |
| `contrib/<域>/v1/` | Optional ecosystem sections consumed by contrib packages or business bootstrap code. | `db/redis` / `kafka` |
| `security/<域>/v1/` | Security primitives shared by runtime packages. | `tls` |
| `obs/<域>/v1/` | Observability runtime configuration consumed by obs packages. | `audit` |
| Neutral schema | CNCF / third-party envelope schemas that do not fit the categories above. | `cloudevents/v1/` |

## Modules

| Package | Description |
|---------|-------------|
| `servora.conf.v1` | 配置注解（扩展号 `5040x`）：布尔 section 标记按段加载，field 声明默认值或必填要求；只生成 `Apply() error`。 |
| `servora.core.v1` | Startup-required framework configuration (Bootstrap and its sub-messages). Loaded by `bootstrap.NewRuntime`; business code reads it via `runtime.Bootstrap`. |
| `servora.obs.audit.v1` | Audit emitter contract (`AuditContract`). |
| `servora.transport.http.cors.v1` | HTTP CORS 配置。启用时，来源、方法和请求头的省略或空列表使用中间件默认值；非空列表覆盖。max_age 默认值由 Proto 声明并经 Apply 补齐。 |
| `servora.contrib.db.redis.v1` | Redis client configuration; section key `redis` (optional), loaded explicitly with `bootstrap.Scan` and consumed by `contrib/db/redis`. |
| `servora.contrib.kafka.v1` | Kafka client configuration; section key `kafka` (optional), loaded explicitly with `bootstrap.Scan` and consumed by `contrib/kafka`. |
| `servora.security.tls.v1` | Shared TLS configuration referenced by core server/client endpoint config and consumed by `security/tls`. |
| `servora.audit.v1` | `servora/audit/v1/annotations.proto` 中的审计注解；供 `protoc-gen-servora-audit` 使用（extension `5010x`，支持 service-level `service_default` + 三态 `AuditMode`；`AuditRule` 只携带 RPC audit 开关）。 |
| `servora.redact.v3` | `servora/redact/v3/redact.proto` 中的 field-level 日志脱敏注解；供 `protoc-gen-redact` 使用，生成非破坏性的 `Redact() string`。 |

## 业务服务加载配置

业务 `main.go` 先加载框架配置，再显式扫描私有配置；段名由消息名称自动推导，例如 AuditContract 对应 audit_contract：

```go
rt, err := bootstrap.NewRuntime(flagconf,
    bootstrap.Name(Name),
    bootstrap.Version(Version),
)
if err != nil {
    return err
}
// 按描述符中的配置段标记扫描，成功后调用 Apply。
// 缺段跳过，不填默认；模块真正使用配置时仍须检查必要参数。
kafka := &kafkapb.Kafka{}
audit := &auditconfpb.AuditContract{}
if err := bootstrap.Scan(rt, kafka, audit); err != nil {
    return errors.Join(err, rt.Close(context.Background()))
}

return rt.Run(func() (*kratos.App, func(), error) {
    return wireApp(rt, kafka, audit)
})
```

## Usage

Add to your `buf.yaml`:

```yaml
deps:
  - buf.build/servora/servora
```

Then run:

```bash
buf dep update
```

## Links

- [GitHub](https://github.com/Servora-Kit/servora)
- [Go Package](https://pkg.go.dev/github.com/Servora-Kit/servora)
