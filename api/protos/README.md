# servora

Shared protobuf definitions for the [Servora](https://github.com/Servora-Kit/servora) microservice framework.

## Layout

Proto files are organized in these top-level groups under `servora/`:

| Group | Purpose | Members |
|-------|---------|---------|
| Annotations (flat) | Extension annotations consumed by `protoc-gen-servora-*` plugins. Each namespace holds a single `annotations.proto`. | `conf/v1` / `errors/v1` |
| `redact/v3/` | Field-level log-redaction annotations consumed by `protoc-gen-redact`. | `redact.proto` |
| `core/v1/` | Framework startup bootstrap model. | `bootstrap.proto` (Bootstrap / App / Server / Data / Registry / Source / Observability) |
| `security/<域>/v1/` | 运行时共享安全配置。 | `tls` |
| `transport/http/<域>/v1/` | HTTP 传输可选配置。 | `apidocs` / `cors` |

## Modules

| Package | Description |
|---------|-------------|
| `servora.conf.v1` | 配置注解（扩展号 `5040x`）：布尔 section 标记按段加载，field 声明默认值或必填要求；只生成 `Apply() error`。 |
| `servora.core.v1` | Startup-required framework configuration (Bootstrap and its sub-messages). Loaded by `bootstrap.NewRuntime`; business code reads it via `runtime.Bootstrap`. |
| `servora.errors.v1` | 错误码注解（扩展号 `5050x`）：声明 enum 默认 HTTP 状态码与枚举值覆盖码，由 `protoc-gen-go-errors` 消费。 |
| `servora.transport.http.apidocs.v1` | HTTP Scalar API 文档配置，由 `server.http.api_docs` 引用。 |
| `servora.transport.http.cors.v1` | HTTP CORS 配置。启用时，来源、方法和请求头的省略或空列表使用中间件默认值；非空列表覆盖。max_age 默认值由 Proto 声明并经 Apply 补齐。 |
| `servora.security.tls.v1` | Shared TLS configuration referenced by core server/client endpoint config and consumed by `security/tls`. |
| `servora.redact.v3` | `servora/redact/v3/redact.proto` 中的 field-level 日志脱敏注解；供 `protoc-gen-redact` 使用，生成非破坏性的 `Redact() string`。 |

## 业务服务加载配置

业务 `main.go` 先加载框架配置，再显式扫描私有配置；段名由消息名称自动推导，例如 CORS 对应 cors：

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
cors := &corsv1.CORS{}
if err := bootstrap.Scan(rt, cors); err != nil {
    return errors.Join(err, rt.Close(context.Background()))
}

return rt.Run(func() (*kratos.App, func(), error) {
    return wireApp(rt, cors)
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
