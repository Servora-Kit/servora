# AGENTS.md - transport/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-22 | Updated: 2026-03-22 -->

## 模块目的

提供服务间 transport 工具箱，覆盖 client / server 装配、连接管理、TLS 与通用 middleware 支撑。

## 当前结构

```text
transport/
├── client/
├── runtime/
├── shared/
└── server/
```

## 当前实现事实

- `client/` 目录承载 `contracts.go`、`factory.go`、`dial_value.go`、`grpc/`、`http/` 与客户端 middleware
- `runtime/` 目录承载插件契约、registry、graph 及默认插件注册（`runtime/defaults`）
- `shared/` 目录承载 tls/endpoint/config 等跨协议复用能力
- `server/` 目录承载 `grpc/`、`http/`、`middleware/`、`plugin.go`、`server.go`、`tls.go`
- `server/middleware/whitelist.go` 的白名单语义是 **operation 白名单**，不是 IP 白名单
- 本级目录表达的是 transport 共性能力，不直接承载认证/授权业务本身

## 边界约束

- 本包负责传输层装配与协议辅助，不负责业务 handler、资源授权策略或领域规则
- `security/authn` / `security/authz` 可基于 transport middleware 工作，但身份与权限语义不应反向塞入 transport 基础设施
- 不在本级目录递归描述 `client/`、`server/` 子目录内部细节；需要更细规则时应在子目录独立维护

## 常见反模式

- 在 transport 目录中写入业务 handler 或 service 级逻辑
- 将 operation 白名单误解为网络访问控制白名单
- 把 client / server 共性抽象与某个协议实现强耦合

## 测试与使用

```bash
go test ./transport/...
go test ./transport/client/...
go test ./transport/server/...
```

## 维护提示

- 若新增 transport middleware，优先保证其可复用且与业务语义解耦
- 若调整 client / server 目录边界，需同步检查父级 `servora/AGENTS.md` 与调用方引用说明
