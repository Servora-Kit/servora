# AGENTS.md - pkg/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-15 | Updated: 2026-03-15 -->

## 目录概览

`pkg/` 提供跨服务复用的基础能力。当前子目录：`actor`、`bootstrap`、`ent/mixin`、`governance`、`health`、`helpers`、`jwks`、`jwt`、`k8s`、`logger`、`mapper`、`openfga`、`redis`、`transport`。

## 模块速览

| 目录 | 用途 |
|------|------|
| `actor/` | 请求上下文中的调用方抽象 |
| `bootstrap/` | 启动链路与配置加载 |
| `ent/mixin` | Ent schema 混入 |
| `governance/` | 注册发现、配置中心、遥测 |
| `health/` | 健康检查 |
| `helpers/` | 通用辅助与 bcrypt 哈希 |
| `jwks/` | JWKS 解析 |
| `jwt/` | JWT 工具与 context 注入 |
| `k8s/` | Kubernetes 客户端 |
| `logger/` | Kratos + Zap，含 GORM/Ent 桥接 |
| `mapper/` | 模型映射 |
| `openfga/` | OpenFGA 客户端与授权 |
| `redis/` | Redis 客户端与锁/Cache-aside |
| `transport/` | 服务间 transport client |

## 当前事实

- `governance/` 下已经是 `config/`、`registry/`、`telemetry/`，旧的 `configCenter/` 描述已失效
- `logger/` 除 `gorm_log.go` 外还有 `ent_log.go`
- `middleware/whitelist.go` 实现的是 **operation 白名单**，不是 IP 白名单
- `redis/` 当前目录没有单独测试文件

## 开发约定

- 优先保持无状态、可复用、低耦合
- 需要资源释放时返回 `cleanup func()`
- 不在库代码里 `panic` 或 `log.Fatal`
- 依赖生成配置类型时，从 `api/gen/go/conf/v1` 导入

## 常用命令

```bash
go test ./pkg/...
go test ./pkg/logger/...
go test ./pkg/governance/registry/...
go test ./pkg/redis/...
```

## 维护提示

- 若更新共享基础设施能力，优先同步本文件与对应子模块 AGENTS.md
- `helpers` 承担密码哈希等通用辅助
