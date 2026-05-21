# AGENTS.md - transport/client/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-05-21 -->

## 模块定位

`transport/client` 提供服务调用侧工具：gRPC/HTTP dialer、按 protocol 的 endpoint 索引、标准 client middleware chain。

本目录负责连接装配和通用中间件，不负责业务 SDK、认证策略、授权策略或具体下游 API 封装。

## 子目录职责

| 目录 | 职责 |
| --- | --- |
| `endpoint/` | 从 `corev1.Data.Client.Services` 构建 service -> endpoint 配置索引 |
| `grpc/` | gRPC dialer；支持 discovery endpoint、timeout、middleware、TLS |
| `http/` | HTTP client dialer；支持直接 URL、discovery endpoint、timeout、middleware |
| `middleware/` | 标准 client chain：recovery、tracing、logging、circuitbreaker、metrics |

## 连接语义

- `endpoint.IndexByProtocol` 会 trim/lower protocol；service name 为空或同一 service/protocol 重复配置时报错。
- gRPC 默认 target 是 `discovery:///<service>`，默认 timeout 5s；配置 endpoint/timeout/TLS 时覆盖默认。
- gRPC TLS 构造通过 `security/tls` alias，不在本目录读取证书文件。
- HTTP target 如果是完整 URL 直接使用；否则默认 `discovery:///<target>`。
- discovery 只在 endpoint 使用 `discovery:///` 时接入。

## Middleware chain

`middleware.NewChainBuilder(l).Build()` 默认包含 recovery、可选 tracing、logging、默认 circuitbreaker、可选 metrics。

默认链不包含 JWT 透传。需要出站 Bearer token 传播时，调用方显式 `append(ms, authjwt.Client())`。不要在默认链里自动加入任何凭据透传逻辑。

## 常见反模式

- 在 client dialer 中硬编码业务 service 名或 API path。
- 绕过 `endpoint.IndexByProtocol` 自行重复解析 `Data.Client.Services`。
- 在 HTTP/gRPC dialer 里复制 TLS 证书加载逻辑。
- 默认透传入站身份到所有下游。

## 测试

```bash
go test ./transport/client/...
```

修改 endpoint 配置 shape 时，同时检查 `api/protos/servora/core/v1`、`core/bootstrap` 与 gRPC/HTTP dialer 测试。
