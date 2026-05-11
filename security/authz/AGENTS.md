# AGENTS.md - security/authz/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-22 | Updated: 2026-05-11 -->

## 模块目的

提供接口驱动的授权中间件框架，消费 protoc 生成的 `AuthzRule`，在请求进入业务层前完成资源级授权判定。引擎实现（OpenFGA、Noop 等）通过 `Authorizer` 接口注入，中间件层本身不依赖任何具体授权后端。每次 Check 决策（allow / deny / error 三态）以 `*auditpb.AuthzDetail` 形式 push 到 ctx，由末端 `audit.Collector` 单点 emit。

## 目录结构

```
security/authz/
  authz.go          → Authorizer 接口 + AuthzRule + Server() 中间件 + Option
  authz_test.go     → 中间件层测试（使用 fakeAuthorizer）
  doc.go            → 包级 godoc
  openfga/
    authorizer.go   → Authorizer 实现（authz.Authorizer + batch.BatchAuthorizer + lister.Lister）
    client.go       → 底层 OpenFGA SDK 客户端构造
    check.go        → 底层关系检查封装
    list.go         → 底层列表查询封装
    tuples.go       → tuple 写入/删除 + 审计
    cache.go        → Redis 缓存层
    config.go       → NewClientOptional 便捷构造
  batch/
    batch.go        → BatchAuthorizer 子接口
  lister/
    lister.go       → Lister 子接口
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
    pkgauthz.WithRulesFunc(iampb.AuthzRules),
)
```

审计事件由 transport 链外层的 `audit.Collector(recorder)` 自动 emit——authz middleware 内部已经把每次 Check 决策（allow / deny / error）写到 ctx，业务侧**无需**额外接线。详见 [`../../obs/audit/AGENTS.md`](../../obs/audit/AGENTS.md) 的 Mounting位置。

## Audit ctx 写入路径

`Server()` 在 `Authorizer.Check` 返回后（无论 allow / deny / error），调用 `audit.WithAuthzResult(ctx, &auditpb.AuthzDetail{...})` 写入决策。三态映射如下：

| `Authorizer.Check` 返回 | `Decision`                       | `ErrorReason`     |
| ----------------------- | -------------------------------- | ----------------- |
| `(_, err != nil)`       | `AUTHZ_DECISION_ERROR`           | `err.Error()`     |
| `(true, nil)`           | `AUTHZ_DECISION_ALLOWED`         | `""`              |
| `(false, nil)`          | `AUTHZ_DECISION_DENIED`          | `""`              |

错误优先于 deny：授权后端故障（超时、网络错误、内部异常）与策略拒绝在告警 / SLO / 风控信号上含义不同，下游消费方必须能区分两者，不能合并成单一 `denied`。

**ctx 写入发生在返回之前**——deny / error 短路 return 时 ctx 已有 detail，外层 `audit.Collector` LIFO 后置阶段读到并 emit `AUTHZ_DECISION` 事件。

未走到 `Authorizer.Check` 的路径**不写 ctx**（参 `Server()` doc）：

- 无 transport（非 server 调用）→ passthrough
- `AUTHZ_MODE_NONE` → 公开接口直接放行
- 缺 actor / 匿名 actor → `AUTHZ_DENIED`，但 Authorizer 未被调用，没有"决策"可记录
- nil authorizer → `AUTHZ_UNAVAILABLE`
- 缺规则（fail-closed 默认）→ `AUTHZ_NO_RULE`

写入时同时挂 OTel span event `audit.authz.recorded`；OTel 未配 / 当前 span 未采样时 SDK noop，零开销。

## 依赖方向

`security/authz` 主包仅 import 中立 schema 包 `api/gen/go/servora/audit/v1`、`api/gen/go/servora/authz/v1` 与 `obs/audit` 的 ctx helper（`WithAuthzResult`）；不 import emitter / recorder 实现。authz 无法感知最终事件落到哪里——这是 v0.4.4 push-ctx + 末端 collector 单点 emit 的核心设计。

## 当前实现事实

- `Server()` 根据 operation 查找 `AuthzRule`，按规则模式执行授权判定
- `AuthzMode_AUTHZ_MODE_NONE` → 直接放行（公开接口）
- `AuthzMode_AUTHZ_MODE_CHECK` → 调用 `Authorizer.Check()`
- `AuthzRule.Mode` 引用共享 proto `api/gen/go/servora/authz/v1`（非 IAM 服务 proto）
- `openfga.Authorizer` 实现三接口：`authz.Authorizer` / `batch.BatchAuthorizer` / `lister.Lister`
- Redis 缓存为 openfga 内部关注点（`WithRedisCache` 选项注入）
- `WithCheckTimeout(d)` 限制后端调用时长；`WithFailOpenOnMissingRule(alertFn)` 开发期可放行未注册 RPC 并回调告警
- `extractProtoField` 支持 dot-path（`parent.id`），路径中段必须为单 message，终点必须为标量
- principal 构造为 `<actor.Type()>:<actor.ID()>`（与 OpenFGA SDK 语义对齐）

## 边界约束

- 本包负责授权执行策略，不负责模型设计、关系写入或 OpenFGA store 运维
- 不在本包定义业务常量、组织树规则或资源生命周期
- 本包对 `obs/audit` 的依赖仅限 ctx helper `WithAuthzResult`；不引入 emitter / recorder
- 新增授权引擎只需实现 `Authorizer` 接口，放入 `security/authz/<engine>/` 子目录

## 常见反模式

- 在 middleware 中硬编码业务资源规则，绕过生成的 `AuthzRule`
- 缺少规则时默认放行，导致权限面失控
- 把对象解析、授权决策、业务补偿逻辑揉在一起
- 在主包 import `obs/audit` 的 emit 实现（破坏 push-ctx + 末端 emit 的单点设计）
- 把 cache 命中作为审计语义暴露给 middleware（cache 是 `security/authz/openfga` 内部关注点）

## 测试与使用

```bash
go test ./security/authz/...
```

## 维护提示

- 若 proto AuthZ 注解有变更，先执行根目录 `make gen` 再检查本包调用链
- `AuthzRule` 的 `Mode` 字段类型为 `authzpb.AuthzMode`，来自 `api/gen/go/servora/authz/v1`（不是 IAM service proto）
- 若新增授权引擎，在 `security/authz/<engine>/` 建子目录，实现 `authz.Authorizer`
- 若 `auditpb.AuthzDetail` 字段调整，先改 proto + `make gen`，本包 ctx 写入处随 schema 自动更新
