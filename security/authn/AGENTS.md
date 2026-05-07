# AGENTS.md - security/authn/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-22 | Updated: 2026-05-07 -->

## 模块目的

提供接口驱动的认证中间件框架，负责从请求中提取 Bearer Token，委托 `Authenticator` 实现完成身份验证，并将 `actor.Actor` 注入上下文。同时把每次认证结果（成功 / 失败 / 匿名通过）以 `*auditpb.AuthnDetail` 形式 push 到 ctx，供末端 `audit.Collector` 单点 emit。

## 目录结构

```
security/authn/
  authn.go          → Authenticator 接口（含 Method()）+ Server() 中间件 + 中间件级 Option
  authn_test.go     → 中间件层测试（使用 fakeAuthenticator）
  jwt/
    jwt.go          → JWTAuthenticator 实现（读取 TokenFromContext，调用 Verifier；Method() 返回 "jwt"）
    claims.go       → ClaimsMapper 类型 + DefaultClaimsMapper + KeycloakClaimsMapper
    options.go      → JWT 引擎 Option（WithVerifier, WithClaimsMapper）
  noop/
    noop.go         → NoopAuthenticator（总是返回 anonymous actor；Method() 返回 "noop"）
```

## 使用方式

```go
import (
    "github.com/Servora-Kit/servora/security/authn"
    authjwt "github.com/Servora-Kit/servora/security/authn/jwt"
)

// JWT 验证（标准 OIDC claims）
mw = append(mw, authn.Server(
    authjwt.NewAuthenticator(authjwt.WithVerifier(km.Verifier())),
))

// Keycloak 特有 claims 映射
mw = append(mw, authn.Server(
    authjwt.NewAuthenticator(
        authjwt.WithVerifier(km.Verifier()),
        authjwt.WithClaimsMapper(authjwt.KeycloakClaimsMapper()),
    ),
))
```

审计事件由 transport 链外层的 `audit.Collector(recorder)` 自动 emit——authn middleware 内部已经把每次结果写到 ctx，业务侧**无需**额外接线。详见 [`../../obs/audit/AGENTS.md`](../../obs/audit/AGENTS.md) 的 Mounting位置。

## Authenticator 接口契约

接口仅容纳两类成员：

1. **认证行为本体**：`Authenticate(ctx) (actor.Actor, error)`
2. **引擎不变元数据**：`Method() string`（自我描述，固定串）

编排（hook / callback）、注入（logger / tracer）、infra 探测（health）一律不在内——它们属于调用方 / 容器 / 可选接口。这是接口的天花板，不是滑坡起点；未来谁想加 `OnSuccess` / `Refresh` / `Tracer`，凭这条直接驳回。

`Method()` 的返回值由引擎自身决定，是审计事件中 `AuthnDetail.Method` 字段的 source of truth：

| 引擎                                          | `Method()` 返回 |
| --------------------------------------------- | --------------- |
| `security/authn/jwt.JWTAuthenticator`         | `"jwt"`         |
| `security/authn/noop.NoopAuthenticator`       | `"noop"`        |
| `security/authn/mtls.*`（P1-3 范围，未实现）  | `"mtls"`        |

middleware 在构造期 cache 一次 `Method()`（不是每请求 dispatch），写 ctx 时直接读取 cached 值，杜绝硬编码 `"jwt"`。

## Audit ctx 写入路径

`Server()` 内部在以下三种结果点都会调用 `audit.WithAuthnResult(ctx, &auditpb.AuthnDetail{...})`：

| 场景                              | `Success` | `FailureReason`        |
| --------------------------------- | --------- | ---------------------- |
| 无 transport（非 server 调用）    | `true`    | `""`（匿名通过）        |
| `Authenticator.Authenticate` 成功 | `true`    | `""`                   |
| `Authenticator.Authenticate` 失败 | `false`   | `err.Error()`          |

**写入发生在返回 / 调用 errorHandler 之前**——authn 失败短路 return 时，ctx 已有 detail，外层 `audit.Collector` LIFO 后置阶段读到并 emit `AUTHN_RESULT` 事件（参 `e2e_test.go` 失败路径覆盖）。

写入时同时挂 OTel span event `audit.authn.recorded`；OTel 未配 / 当前 span 未采样时 SDK noop，零开销。

覆盖范围：

- 当前覆盖 JWT 引擎与 Noop 引擎的全部成功 / 失败 / 匿名通过路径
- P1-3 mTLS app-layer（SAN / XFCC 解析失败等）后续接入同一 ctx-write 路径
- **不**覆盖 TLS handshake 错误——握手层错误属于 transport 层指标 + 日志范畴，不在 authn middleware 视野内

## 依赖方向

`security/authn` 主包仅 import 中立 schema 包 `api/gen/go/servora/audit/v1` 与 `obs/audit` 的 ctx helper（`WithAuthnResult` 等）；不 import emitter / recorder 实现。authn 无法感知最终事件落到哪里——这是 v0.4.4 push-ctx + 末端 collector 单点 emit 的核心设计。

## 当前实现事实

- `Server()` 从 Authorization header 提取 Bearer token 存入 `svrmw.TokenContext`，再调用 `Authenticator.Authenticate(ctx)` 获取 actor
- JWT 引擎从 `svrmw.TokenFromContext(ctx)` 读取 token，完成 Verifier 校验后使用 `ClaimsMapper` 映射为 actor
- `DefaultClaimsMapper` 仅映射标准 OIDC claims（sub, name, email, azp, scope），不含 IdP 特有字段
- `KeycloakClaimsMapper` 在 DefaultClaimsMapper 基础上额外映射 `iss→Realm`
- `NoopAuthenticator` 总是返回 anonymous actor，`Method()` 返回 `"noop"`，用于测试或无需认证的服务

## 边界约束

- 本包是 middleware 层，不是 JWT 基础库；签发 / 验签细节在 `security/jwt`
- 新增认证引擎只需实现 `Authenticator` 接口（`Authenticate` + `Method`），放入 `security/authn/<engine>/` 子目录
- 本包不承载业务 claims 解释规则（租户、项目等）
- 本包不做资源级授权；授权决策在 `security/authz`
- 不在主包 import emit 实现，仅 import `audit.WithAuthnResult` ctx helper

## 常见反模式

- 在 `security/authn` 中堆积业务 claims 解释和领域规则
- 把匿名身份、缺 token、验签失败三种状态混成一种处理
- 绕过 `actor` / transport context，直接在业务层重复解析 token
- 在 jwt/ 子包中直接 import 父包（会造成循环依赖）
- 中间件内部硬编码 `Method:"jwt"`（应读 `authenticator.Method()`）
- 在 `Authenticator` 接口加新方法（hook / callback / probe）—— 违反 contract 注释

## 测试与使用

```bash
go test ./security/authn/...
```

## 维护提示

- 若调整 ClaimsMapper 字段，需同步检查 `core/actor` 接口契约
- 若新增认证引擎，在 `security/authn/<engine>/` 建子目录，实现 `authn.Authenticator`（含 `Method()`）
- JWT 引擎依赖 `svrmw.TokenFromContext`，确保 `Server()` 在引擎 `Authenticate()` 前已写入 token context
- 若 `auditpb.AuthnDetail` 字段调整，先改 proto + `make gen`，本包 ctx 写入处随 schema 自动更新
