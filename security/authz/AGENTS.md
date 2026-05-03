# AGENTS.md - security/authz/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-22 | Updated: 2026-03-23 -->

## 模块目的

提供接口驱动的授权中间件框架，消费 protoc 生成的 `AuthzRule`，在请求进入业务层前完成资源级授权判定。引擎实现（OpenFGA、Noop 等）通过 `Authorizer` 接口注入，中间件层本身不依赖任何具体授权后端。

## 目录结构

```
security/authz/
  authz.go          → Authorizer 接口 + AuthzRule + DecisionDetail + Server() 中间件 + Option
  authz_test.go     → 中间件层测试（使用 fakeAuthorizer）
  openfga/
    openfga.go      → OpenFGAAuthorizer 实现（封装 infra/openfga.Client + 可选 Redis 缓存）
    options.go      → OpenFGA 引擎 Option（WithRedisCache）
  noop/
    noop.go         → NoopAuthorizer（总是放行，用于测试）
```

## 使用方式

```go
import (
    pkgauthz "github.com/Servora-Kit/servora/security/authz"
    fgaengine "github.com/Servora-Kit/servora/security/authz/openfga"
)

// OpenFGA 授权（可选 Redis 缓存）
authzMw := pkgauthz.Server(
    fgaengine.NewAuthorizer(fgaClient,
        fgaengine.WithRedisCache(rdb, openfga.DefaultCheckCacheTTL),
    ),
    pkgauthz.WithRulesFunc(iamv1.AuthzRules),
)

// 桥接审计事件（DecisionLogger 回调）
authzMw := pkgauthz.Server(authorizer,
    pkgauthz.WithRulesFunc(rules),
    pkgauthz.WithDecisionLogger(func(ctx context.Context, d pkgauthz.DecisionDetail) {
        recorder.RecordAuthzDecision(ctx, d.Operation, actor, ...)
    }),
)
```

## 当前实现事实

- `Server()` 根据 operation 查找 `AuthzRule`，按规则模式执行授权判定
- `AuthzMode_AUTHZ_MODE_NONE` → 直接放行（公开接口）
- `AuthzMode_AUTHZ_MODE_CHECK` → 调用 `Authorizer.Check()`
- `AuthzRule.Mode` 引用共享 proto `api/gen/go/servora/authz/v1`（非 IAM 服务 proto）
- 审计发射通过 `WithDecisionLogger` 回调实现；中间件本身不 import `obs/audit`
- `DecisionDetail` 包含 `Operation`、`Subject`、`Relation`、`ObjectType`、`ObjectID`、`Allowed`、`Err`（cache 命中不进审计语义，留在 `infra/openfga` 内部）
- `OpenFGAAuthorizer` 封装 Redis 缓存为内部关注点（`WithRedisCache`）
- `Authorizer` 接口含三方法：`Check` / `BatchCheck` / `ListAllowed`，openfga 与 noop 完整覆盖
- `WithCheckTimeout(d)` 限制后端调用时长；`WithFailOpenOnMissingRule(alertFn)` 开发期可放行未注册 RPC 并回调告警
- `extractProtoField` 支持 dot-path（`parent.id`），路径中段必须为单 message，终点必须为标量
- `NewAuthzBridge(recorder)` 一键把 decision 落到 `obs/audit` Recorder（独立 `bridge.go`，主中间件仍不依赖 `obs/audit`）

## 边界约束

- 本包负责授权执行策略，不负责模型设计、关系写入或 OpenFGA store 运维
- 不在本包定义业务常量、组织树规则或资源生命周期
- 审计通过 `WithDecisionLogger` 回调注入，本包不依赖 `obs/audit`
- 新增授权引擎只需实现 `Authorizer` 接口，放入 `security/authz/<engine>/` 子目录

## 常见反模式

- 在 middleware 中硬编码业务资源规则，绕过生成的 `AuthzRule`
- 缺少规则时默认放行，导致权限面失控
- 把对象解析、授权决策、业务补偿逻辑揉在一起
- 把 `obs/audit` 等具体依赖直接注入到 `security/authz`

## 测试与使用

```bash
go test ./security/authz/...
```

## 维护提示

- 若 proto AuthZ 注解有变更，先执行根目录 `make api` 再检查本包调用链
- `AuthzRule` 的 `Mode` 字段类型为 `authzpb.AuthzMode`，来自 `api/gen/go/servora/authz/v1`（不是 IAM service proto）
- 若新增授权引擎，在 `security/authz/<engine>/` 建子目录，实现 `authz.Authorizer`
