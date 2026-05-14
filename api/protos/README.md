# servora

Shared protobuf definitions for the [Servora](https://github.com/Servora-Kit/servora) microservice framework.

## Layout

Proto files are organized in four top-level groups under `servora/`:

| Group | Purpose | Members |
|-------|---------|---------|
| Annotations (flat) | Extension annotations consumed by `protoc-gen-servora-*` plugins. Each namespace holds a single `annotations.proto`. | `audit/v1` / `authn/v1` / `authz/v1` / `mapper/v1` / `conf/v1` |
| `core/v1/` | Framework startup-required configuration — missing fields here would block boot. | `bootstrap.proto` (Bootstrap / App / Server / Registry / Discovery / Config / Data / Trace / Metrics / TLSConfig) |
| `extra/<域>/v1/` | Per-domain optional configuration. Each domain lives in its own namespace and proto file, loaded via `bootstrap.ScanSections`. | `broker` / `audit` / `cors` / `mail` / `jwt` |
| Neutral schema | CNCF / third-party envelope schemas that do not fit the categories above. | `cloudevents/v1/` |

## Modules

| Package | Description |
|---------|-------------|
| `servora.conf.v1` | Annotation extensions for `protoc-gen-servora-conf` (extension `5040x`, message-level `section` + field-level `field` rules driving `SectionKey` / `ApplyDefaults` / `ValidateConf` generation) |
| `servora.core.v1` | Startup-required framework configuration (Bootstrap and its sub-messages). Loaded via `bootstrap.ScanConf[corev1.Bootstrap]` or directly via `runtime.Bootstrap`. |
| `servora.extra.broker.v1` | Message broker configuration (Kafka backend); `Broker` is an optional section under key `broker`. |
| `servora.extra.audit.v1` | Audit emitter contract (`AuditContract`); section key `audit`. |
| `servora.extra.cors.v1` | HTTP CORS middleware configuration; section key `cors` (optional). Defaults sourced from the proto annotation, applied via generated `ApplyDefaults()`. |
| `servora.extra.mail.v1` | SMTP / mail configuration retained for IAM compatibility; section key `mail` (optional). |
| `servora.extra.jwt.v1` | JWT issuer / verifier configuration retained for IAM compatibility; section key `jwt` (optional). |
| `servora.pagination.v1` | Pagination request/response messages. |
| `servora.mapper.v1` | Object mapping annotation extensions for `protoc-gen-servora-mapper` (extension numbers `5000x`). |
| `servora.audit.v1` | Audit annotation extensions for `protoc-gen-servora-audit` (extension `5010x`, supports service-level `service_default` + three-state `AuditMode` / `ErrorRecordMode` enums). |
| `servora.authz.v1` | Authorization annotation extensions for `protoc-gen-servora-authz` (extension `5020x`, supports service-level `service_default`). |
| `servora.authn.v1` | Authentication annotation extensions for `protoc-gen-servora-authn` (extension `5030x`, supports service-level `service_default` + `schemes` / `credentials_locations` fields). |

## Loading configuration in business services

Business `main.go` typically loads framework + domain configuration in two steps:

```go
err := bootstrap.BootstrapAndRun(flagconf, Name, Version, func(rt *bootstrap.Runtime) (*kratos.App, func(), error) {
    bc := rt.Bootstrap // *corev1.Bootstrap, populated by core/v1 yaml fields

    // Optional extra sections — each implements the Section interface generated
    // by protoc-gen-servora-conf. ScanSections invokes ApplyDefaults + ValidateConf
    // automatically; missing optional sections are skipped silently.
    broker := &brokerv1.Broker{}
    audit  := &auditcontractv1.AuditContract{}
    if err := bootstrap.ScanSections(rt, broker, audit); err != nil {
        return nil, nil, err
    }

    return wireApp(bc.Server, bc.App, /* ... */, broker, audit, rt.Identity, rt.Logger)
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
