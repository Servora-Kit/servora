# AGENTS.md - security/authn/jwt/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-05-10 -->

## 子包定位

`security/authn/jwt` 是**通用 Bearer JWT 认证骨架**——只承担「读 Bearer + 验签 + claim 三件套」的最小骨架，不绑定任何 IdP（Keycloak / Auth0 / Okta / Cognito / 自建 IdP 通通由业务在外层通过 `ClaimsMapper` 注入）。

四件套自治：

- **Authenticator**（验签 + claims → actor 三件套）
- **Server wrapper**（power-user 单引擎直挂便利包装）
- **Client middleware**（出站 Bearer 透传）
- **引擎私有 ctx 通道**（raw token 流转）

framework 主层（`security/authn`）对 "Bearer token" 这个字眼无感知；其他引擎（mtls / apikey / passkey）应当仿照本包结构在各自 `security/authn/<engine>/` 子目录复制粘贴模板。

## 目录结构

```
security/authn/jwt/
  jwt.go            → NewAuthenticator + Authenticate + Scheme 常量 + token 解析优先级
  server.go         → Server(opts...) 单引擎直挂便利包装（authn.Server + authn.Multi 一行套）
  client.go         → Client() 出站透传中间件
  context.go        → WithToken / TokenFrom（jwt 私有 ctx 通道）
  extract.go        → extractBearerToken（包私有，从历史公开 API 收回）
  options.go        → WithVerifier / WithClaimsMapper
  claims.go         → ClaimsMapper + DefaultClaimsMapper（仅三件套）
  jwt_test.go       → wrapper / Client / chain 短路 / Authenticate / ClaimsMapper 全路径
  extract_test.go   → 表驱动 9 case 覆盖 Bearer 解析边界
```

## 公开 API

```go
// 包级常量（与 proto schemes / authn.Named 配对）
const Scheme = "jwt"

// 业务首选 entry：单引擎直挂便利包装
func Server(opts ...Option) middleware.Middleware

// 出站透传 jwt token 到 Authorization header
func Client() middleware.Middleware

// 高级扩展点：拿裸引擎自己组装（多引擎 Multi 场景 / 非 HTTP 载体场景）
func NewAuthenticator(opts ...Option) authn.Authenticator

// jwt 私有 ctx 通道（raw token 流转）
func WithToken(ctx context.Context, token string) context.Context
func TokenFrom(ctx context.Context) (string, bool)

// Options
func WithVerifier(v *security/jwt.Verifier) Option
func WithClaimsMapper(m ClaimsMapper) Option

// ClaimsMapper（扩展点）+ 默认实现（仅三件套）
type ClaimsMapper func(claims gojwt.MapClaims) (actor.Actor, error)
func DefaultClaimsMapper() ClaimsMapper
```

## 单引擎 vs 多引擎

### 单引擎直挂（80% 用例）：用 `jwt.Server`

```go
mw = append(mw, jwt.Server(jwt.WithVerifier(km.Verifier())))
```

`jwt.Server(opts...)` 等价于：

```go
authn.Server(
    authn.Multi(
        authn.Named(jwt.Scheme, jwt.NewAuthenticator(opts...)),
    ),
)
```

外加一个**前置步骤**：从入站 `Authorization: Bearer <token>` header 解出 raw token，写入 jwt 私有 ctx 通道（`WithToken`），让 `Client()` / 业务 middleware 后续可以直接 `TokenFrom(ctx)` 读到。

### 多引擎组合（推荐生产姿态）：直接用 `authn.Server` + `authn.Multi`

```go
mw = append(mw, authn.Server(
    authn.Multi(
        authn.Named(jwt.Scheme,    jwt.NewAuthenticator(jwt.WithVerifier(v))),
        authn.Named(apikey.Scheme, apikey.NewAuthenticator(...)),
    ),
    authn.WithRulesFuncs(examplev1.AuthnRules),
))
```

业务多引擎时**不要**用 `jwt.Server`——它只装了 jwt 一个 `Named`，再叠 mtls / apikey 会重复装配。

## token 解析优先级

`Authenticate(ctx)` 按以下顺序解析 raw token：

1. **jwt 私有 ctx 通道**：`TokenFrom(ctx)` 非空则用之（`jwt.Server` 前置步骤的产物，或上游业务显式 `WithToken` 写入）。
2. **Kratos 入站 transport header fallback**：`transport.FromServerContext(ctx).RequestHeader().Get("Authorization")` → `extractBearerToken`。这条 fallback 让多引擎 wiring（不走 `jwt.Server` 而是 `authn.Multi(authn.Named(jwt.Scheme, jwt.NewAuthenticator(...)))`）也能直接工作，**无需**业务自己 pre-write ctx。

两路都拿不到 → 返回 `actor.NewAnonymousActor(), nil`（pass-through 模式）。

## ClaimsMapper：默认仅三件套

`DefaultClaimsMapper()` 只映射 JWT 标准 claim 到 actor 三件套：

| claim 来源                         | actor.UserActor 字段     | 备注                                  |
| ---------------------------------- | ------------------------ | ------------------------------------- |
| `sub`                              | `ID`                     | **必需**——空时返回 error              |
| `name` 或 `preferred_username`     | `DisplayName`            | 优先 `name`，缺失则 fallback          |

**不**做任何 IdP 特化字段（`azp` / `scope` / `email` / `roles` / `groups` / 任何嵌套 claim）。所有富 claim 解析都通过 `WithClaimsMapper(myMapper)` 注入业务自己的 mapper：

```go
custom := func(claims gojwt.MapClaims) (actor.Actor, error) {
    sub, _ := claims["sub"].(string)
    // ...业务自己解释 roles / tenant / 自定义 claim...
    return actor.NewUserActor(sub, displayName), nil
}

auth := jwt.NewAuthenticator(jwt.WithVerifier(v), jwt.WithClaimsMapper(custom))
```

## `Client()` 出站透传

```go
clientMS = append(clientMS, jwt.Client())
```

每请求：

1. 从 jwt 私有 ctx 通道读 token：`tok, ok := TokenFrom(ctx)`
2. ctx 缺 token / token 空 → passthrough（不报错）
3. 没有 client transport 附着 → passthrough（不报错）
4. 否则：`tr.RequestHeader().Set("Authorization", "Bearer "+tok)`

**业务必须显式 `Append(jwt.Client())`**——client 默认链不再自动添加。原因：不是每个出站调用都想透传入站凭据（跨 realm 调用、第三方集成、降权调用等）。

## 私有 ctx 通道（context.go）

```go
type tokenKey struct{}                    // 包私有
func WithToken(ctx, token) context.Context  // 写入
func TokenFrom(ctx) (string, bool)        // 读取
```

`tokenKey` 是包私有结构体；`WithToken / TokenFrom` 函数虽**导出**但语义只服务于：

- jwt 子包内部（`Server` 前置写、`Client` 读）
- 上游显式重新注入场景（如 retry / 网关层 token 翻译）
- 业务 middleware 想观察入站 token

**其他引擎子包应当提供自己的 ctx 通道**：

- `mtls.WithCert(ctx, cert) / mtls.CertFrom(ctx)`
- `apikey.WithKey(ctx, key) / apikey.KeyFrom(ctx)`

通用 transport middleware 包**禁止**承载这类引擎特化的 ctx helper——载体形态是引擎关切，不是框架关切。

## 包私有 `extractBearerToken`

```go
func extractBearerToken(header string) string   // 不导出
```

Bearer 解析逻辑（RFC 6750 风格）：

- 空 header → `""`
- 大小写不敏感 scheme（`Bearer` / `bearer` / `BEARER` 都接受）
- 多余空白 trim（`"Bearer  abc"` → `"abc"`）
- whitespace-only 或只有 scheme 的输入 → `""`
- 错误 scheme（`Basic xyz`）→ `""`
- 缺 scheme prefix → `""`

业务代码**永远不该**直接调这个函数——这是从历史公开 API `authn.ExtractBearerToken` 收回的。

## `Scheme` 常量

```go
const Scheme = "jwt"
```

跟 `protoc-gen-servora-authn` 输出的 `AuthnRule.schemes` 自由文本一一对应。这个值通过 `authn.Named(jwt.Scheme, ...)` 喂给 `authn.Multi`，最终落到 `*auditpb.AuthnDetail.Method` 字段。

**框架不枚举 authn 类型**——其他引擎子包用自己的 `Scheme` 常量（`mtls.Scheme = "mtls"` / `apikey.Scheme = "apikey"` / 业务自定义任意值）。

## 典型业务用法

```go
import (
    "github.com/Servora-Kit/servora/security/authn"
    authjwt "github.com/Servora-Kit/servora/security/authn/jwt"
    pkgmw "github.com/Servora-Kit/servora/transport/server/middleware"
    pkgmwclient "github.com/Servora-Kit/servora/transport/client/middleware"
)

// Server 端：单引擎直挂（仅三件套 claim 映射）
ms := pkgmw.NewChainBuilder(httpLogger).WithAudit(rec).Build()
ms = append(ms, authjwt.Server(authjwt.WithVerifier(km.Verifier())))

// Server 端：业务自定义 ClaimsMapper（解释 roles / tenant / 自定义 claim）
ms = pkgmw.NewChainBuilder(httpLogger).WithAudit(rec).Build()
ms = append(ms, authjwt.Server(
    authjwt.WithVerifier(km.Verifier()),
    authjwt.WithClaimsMapper(myBusinessMapper),
))

// Server 端：多引擎组合（jwt + apikey）
ms = pkgmw.NewChainBuilder(httpLogger).WithAudit(rec).Build()
ms = append(ms, authn.Server(
    authn.Multi(
        authn.Named(authjwt.Scheme,    authjwt.NewAuthenticator(authjwt.WithVerifier(v))),
        authn.Named(apikey.Scheme,     apikey.NewAuthenticator(...)),
    ),
    authn.WithRulesFuncs(examplev1.AuthnRules),
))

// Client 端：显式出站透传 jwt token
clientMS := pkgmwclient.NewChainBuilder(clientLogger).Build()
clientMS = append(clientMS, authjwt.Client())
```

> **注意**：`pkgmw.ChainBuilder` / `pkgmwclient.ChainBuilder` 都没有 fluent `Append` 方法；业务侧用 Go 内建 `append(ms, mw...)` 拼接 jwt wrapper。

## Authenticate 行为

```go
func (a *authenticator) Authenticate(ctx context.Context) (actor.Actor, error)
```

- ctx + transport 都没 token → 返回 `actor.NewAnonymousActor(), nil`（pass-through）
- 没配 Verifier → 返回匿名（pass-through 模式，便于本地 / 测试）
- Verifier 校验失败 → 返回 `nil, fmt.Errorf("jwt: verify token: %w", err)`
- ClaimsMapper 失败（如 `sub` 为空）→ 返回 mapper 自己的 error
- 校验成功 + claim 映射成功 → 返回 actor

## 测试覆盖

```bash
GOWORK=off go test ./security/authn/jwt
```

- **`extract_test.go`**：9 case 表驱动覆盖 `extractBearerToken` 边界
- **`jwt_test.go`** 涵盖：
  - `TestServer_ExtractsBearerAndDispatches` — wrapper happy path（提 Bearer + 写 ctx + 委派 + AuthnDetail.Method = "jwt"）
  - `TestServer_ChainShortCircuit_PassthroughOnExistingActor` — 多机制 chain 短路
  - `TestServer_NoBearerHeader_ReachesAuthenticator` — 缺 header 时仍委派 dispatcher
  - `TestClient_PropagatesToken` — 4 case：with-token / no-token-in-ctx / empty-token / no-client-transport
  - `TestAuthenticate_NoTokenInContext_ReturnsAnonymous` / `TestAuthenticate_NoVerifier_ReturnsAnonymous` — 引擎层匿名路径
  - `TestAuthenticate_TransportHeaderFallback` — 不走 `jwt.Server` 时引擎自取 header
  - `TestTokenForAuth_PrefersCtxOverHeader` / `_FallsBackToHeader` / `_EmptyEverywhere` — token 解析优先级
  - `TestDefaultClaimsMapper_*` — sub+name / preferred_username fallback / name 覆盖 / 仅 sub / 空 sub 报错（5 case）
  - `TestWithClaimsMapper_CustomExtensionPoint` — 自定义 mapper 注入 + 错误传播
  - `TestScheme_IsExposedConstant` — 常量值哨兵

## 边界约束

- 本包是 **JWT 引擎认证骨架**，不是 JWT 基础库；签发 / 验签的密码学细节在 `security/jwt`
- 不做 IdP 特化 claim 解释（Keycloak / Auth0 / Okta / 自建 IdP 全部由业务通过 `WithClaimsMapper` 注入）
- 不做业务 claim 解释规则（租户、项目、roles 等）
- 不做资源级授权（授权在 `security/authz`）
- 不 import `transport/server/middleware/` 或 `transport/client/middleware/` 的引擎特化 helpers
- 不 import 父包 `security/authn` 的内部实现细节，只 import 公开接口（`authn.Authenticator` / `authn.Server` / `authn.Multi` / `authn.Named`）

## 常见反模式

- 在 `transport/server/middleware/` 下重新引入 token-shape 的 ctx helper —— 这是 jwt 引擎私有概念
- 在 `transport/client/middleware/chain.go` 默认 chain 里自动 append `jwt.Client()` —— 业务必须显式 opt-in
- 在业务代码里直接调 `extractBearerToken`（不可访问）—— 用 `jwt.Server(...)` wrapper
- 让其他引擎子包共享 `tokenKey{}` 或 `WithToken / TokenFrom` —— 每个引擎应有自己的 ctx 通道
- 把 `Scheme` 常量值改成 `"bearer"` 或自定义字符串 —— 跟 proto 层 `schemes` 列表强绑定，改名要全链路同步
- 在框架的 `DefaultClaimsMapper` 里加 IdP 字段（roles / email / azp 等）—— 业务自己写 mapper
- 业务多引擎时套 `jwt.Server(...)` 再叠 mtls —— 重复装 `Multi`，应直接 `authn.Server(authn.Multi(authn.Named(...), authn.Named(...)))`

## 维护提示

- 修改 `Server` 或 `Authenticate` 的 token 解析顺序需同步更新 `jwt_test.go` 中 `TestTokenForAuth_*` 系列断言
- 改 `extractBearerToken` 边界行为务必同步 `extract_test.go` 的 9 case
- `Scheme` 常量值（`"jwt"`）跟 proto 层 `servora.authn.v1.AuthnRule.schemes` 自由文本对应；改名前确认下游审计 / 策略消费方的 schemes 配置
- 新增 `Option` 时同步在 `options.go` 加 `With-` 函数 + `authenticatorConfig` 字段；父包 `authn` 不需要任何改动
- 想丰富 `DefaultClaimsMapper` 时——**先停下**：富 claim 是业务关切，骨架包永远只保留三件套
