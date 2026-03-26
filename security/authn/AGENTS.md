# AGENTS.md - security/authn/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-22 | Updated: 2026-03-23 -->

## 模块目的

提供接口驱动的认证中间件框架，负责从请求中提取 Bearer Token，委托 `Authenticator` 实现完成身份验证，并将 `actor.Actor` 注入上下文。

## 目录结构

```
security/authn/
  authn.go          → Authenticator 接口 + Server() 中间件 + 中间件级 Option
  authn_test.go     → 中间件层测试（使用 fakeAuthenticator）
  jwt/
    jwt.go          → JWTAuthenticator 实现（读取 TokenFromContext，调用 Verifier）
    claims.go       → ClaimsMapper 类型 + DefaultClaimsMapper + KeycloakClaimsMapper
    options.go      → JWT 引擎 Option（WithVerifier, WithClaimsMapper）
  noop/
    noop.go         → NoopAuthenticator（总是返回 anonymous actor）
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

## 当前实现事实

- `Server()` 从 Authorization header 提取 Bearer token 存入 `svrmw.TokenContext`，再调用 `Authenticator.Authenticate(ctx)` 获取 actor
- JWT 引擎从 `svrmw.TokenFromContext(ctx)` 读取 token，完成 Verifier 校验后使用 `ClaimsMapper` 映射为 actor
- `DefaultClaimsMapper` 仅映射标准 OIDC claims（sub, name, email, azp, scope），不含 IdP 特有字段
- `KeycloakClaimsMapper` 在 DefaultClaimsMapper 基础上额外映射 `iss→Realm`
- `NoopAuthenticator` 总是返回 anonymous actor，用于测试或无需认证的服务

## 边界约束

- 本包是 middleware 层，不是 JWT 基础库；签发/验签细节在 `security/jwt`
- 新增认证引擎只需实现 `Authenticator` 接口，放入 `security/authn/<engine>/` 子目录
- 本包不承载业务 claims 解释规则（租户、项目等）
- 本包不做资源级授权；授权决策在 `security/authz`

## 常见反模式

- 在 `security/authn` 中堆积业务 claims 解释和领域规则
- 把匿名身份、缺 token、验签失败三种状态混成一种处理
- 绕过 `actor` / transport context，直接在业务层重复解析 token
- 在 jwt/ 子包中直接 import 父包（会造成循环依赖）

## 测试与使用

```bash
go test ./security/authn/...
```

## 维护提示

- 若调整 ClaimsMapper 字段，需同步检查 `core/actor` 接口契约
- 若新增认证引擎，在 `security/authn/<engine>/` 建子目录，实现 `authn.Authenticator`
- JWT 引擎依赖 `svrmw.TokenFromContext`，确保 `Server()` 在引擎 `Authenticate()` 前已写入 token context
