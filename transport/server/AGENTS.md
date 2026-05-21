# AGENTS.md - transport/server/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-05-21 -->

## 模块定位

`transport/server` 提供服务端 transport 装配：统一 `Server` 接口、gRPC/HTTP server 构造、registry endpoint 解析、accept loop 与标准 server middleware chain。

本目录不实现业务 handler，也不承载认证/授权策略；业务中间件通过切片 `append` 挂到 chain 后面。

## 子目录职责

| 目录 | 职责 |
| --- | --- |
| `accept/` | TCP accept 循环辅助 |
| `endpoint/` | 解析 registry endpoint、host、bind addr 与 query |
| `grpc/` | Kratos gRPC server 构造，注册服务，解析 TLS/registry endpoint |
| `http/` | Kratos HTTP server 构造，注册服务、CORS、metrics、health、swagger |
| `middleware/` | 标准 server chain 与 operation whitelist |

顶层 `Server` 接口只聚合 `Start/Stop` 生命周期和 `Endpoint()` 注册地址能力。

## 装配语义

- gRPC/HTTP server 都从 `corev1.Server_*` 读取 listen、timeout、TLS、registry 配置。
- TLS 构造统一调用 `security/tls`；不要在协议子包复制 PEM 解析。
- registry endpoint 解析失败在启动期 panic，避免服务以错误地址注册。
- HTTP 额外负责 CORS、`/metrics`、`/healthz`、`/readyz` 与 swagger 注册。
- 服务 registrar 在 server 创建后执行。

## Middleware chain

`middleware.NewChainBuilder(l).Build()` 固定顺序：recovery、可选 tracing、logging、默认 ratelimit、proto validate、可选 metrics。

返回值是 `[]middleware.Middleware`；没有 fluent `Append`。业务侧用 Go 内建 `append(ms, audit, authn, authz, ...)` 调整后续中间件。

`whitelist` 是 operation whitelist，不是 IP 白名单或网络访问控制。

## 常见反模式

- 在 server 目录写业务 handler 或 service 领域逻辑。
- 把 authn/authz 强塞进默认 chain，导致所有服务不可配置。
- 将 whitelist 当成 IP allowlist。
- 在 HTTP/gRPC 子包各自实现不同的 TLS/registry 解析规则。

## 测试

```bash
go test ./transport/server/...
```

修改 middleware 顺序时，同时检查 audit collector 外层挂载要求和 authn/authz 短路审计路径。
