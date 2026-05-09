# servora

Shared protobuf definitions for the [Servora](https://github.com/Servora-Kit/servora) microservice framework.

## Modules

| Package | Description |
|---------|-------------|
| `servora.conf.v1` | Bootstrap configuration structure |
| `servora.pagination.v1` | Pagination request/response messages |
| `servora.mapper.v1` | Object mapping annotation extensions for `protoc-gen-servora-mapper` (extension numbers `5000x`) |
| `servora.audit.v1` | Audit annotation extensions for `protoc-gen-servora-audit` (extension `5010x`, supports service-level `service_default` + three-state `AuditMode` / `ErrorRecordMode` enums) |
| `servora.authz.v1` | Authorization annotation extensions for `protoc-gen-servora-authz` (extension `5020x`, supports service-level `service_default`) |
| `servora.authn.v1` | Authentication annotation extensions for `protoc-gen-servora-authn` (extension `5030x`, supports service-level `service_default` + `schemes` / `credentials_locations` fields) |

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
