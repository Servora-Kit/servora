# AGENTS.md - security/authn/jwt/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-05-09 -->

## 子包定位

`security/authn/jwt` 是 JWT 引擎对**对称模板**的落地实现：

- Server wrapper（入站 Bearer 提取 + 调度委派）
- Client middleware（出站 Bearer 透传）
- Authenticator（验签 + claims → actor）
- 引擎私有 ctx 通道（raw token 流转载体）

四件套自治——transport 层不持任何 jwt 形态代码，框架其他模块对 "Bearer token" 这个字眼无感知。其他引擎（mtls / apikey / passkey）应当仿照本包结构在各自 `security/authn/<engine>/` 子目录复制粘贴模板。

设计意图详见父包 [`../AGENTS.md`](../AGENTS.md) 的「Wrapper 子包对称模板」章节，以及 [`../../../openspec/changes/authn-engine-agnostic-refactor/design.md`](../../../openspec/changes/authn-engine-agnostic-refactor/design.md) Decisions 4 / 5 / 6 / 8。

## 目录结构

```
security/authn/jwt/
  server.go         → Server(opts...) wrapper（chain 短路 + 提 Bearer + 委派 authn.Server）
  client.go         → Client() 出站透传中间件
  context.go        → WithToken / TokenFromContext（jwt 私有 ctx 通道）
  extract.go        → extractBearerToken（包私有，从历史公开 API 收回）
  jwt.go            → NewAuthenticator + Authenticate 实现
  options.go        → WithVerifier / WithClaimsMapper
  claims.go         → ClaimsMapper + DefaultClaimsMapper + KeycloakClaimsMapper
  jwt_test.go       → wrapper / Client / chain 短路 / Authenticate 全路径
  extract_test.go   → 表驱动 9 case 覆盖 Bearer 解析边界
```

## 公开 API 概览

```go
// 业务首选 entry：完整的 server-side wrapper（80% 用例）
func Server(opts ...Option) middleware.Middleware

// 出站透传 jwt token 到 Authorization header
func Client() middleware.Middleware

// 高级扩展点：拿裸引擎自己组装 wrapper（非 HTTP 载体场景）
func NewAuthenticator(opts ...Option) authn.Authenticator

// jwt 私有 ctx 通道（raw token 流转）
func WithToken(ctx context.Context, token string) context.Context
func TokenFromContext(ctx context.Context) (string, bool)

// Options
func WithVerifier(v *security/jwt.Verifier) Option
func WithClaimsMapper(m ClaimsMapper) Option

// Helpers
func DefaultClaimsMapper() ClaimsMapper      // 标准 OIDC claims（sub, name, email, azp, scope）
func KeycloakClaimsMapper() ClaimsMapper     // Default + iss→Realm + realm_access.roles
```

## `Server(opts...)` wrapper 工作流程

```go
mw = append(mw, jwt.Server(jwt.WithVerifier(km.Verifier())))
```

中间件每请求执行三步（顺序刚性）：

1. **Chain 短路**

   ```go
   if a, ok := actor.FromContext(ctx); ok && a.Type() != actor.TypeAnonymous {
       return handler(ctx, req)
   }
   ```

   ctx 已有非匿名 actor 表示前面的引擎（多机制 chain 场景）已认证成功——直接 passthrough，**不**调引擎、**不**写 AuthnDetail（前一个引擎已经写过）、**不**触碰 actor。

2. **提取 Bearer**

   从 `Authorization: Bearer <token>` header 取原始 token，写入 jwt 私有 ctx 通道：

   ```go
   if tr, ok := transport.FromServerContext(ctx); ok {
       if raw := extractBearerToken(tr.RequestHeader().Get("Authorization")); raw != "" {
           ctx = WithToken(ctx, raw)
       }
   }
   ```

   header 缺失 / 格式不对 → 不写 ctx，引擎读到空 token 走匿名路径。

3. **委派调度**

   ```go
   wrapped(ctx, req)   // wrapped = authn.Server(authenticator, authn.WithMethod(methodName))(handler)
   ```

   主包 `authn.Server` 调 `Authenticate`、写 `AuthnDetail{Method:"jwt", ...}`、注入 actor，然后调用 handler。

`authn.Server` dispatcher 在装配期被构造一次，`wrapped = innerMW(handler)` 也只组合一次（避免每请求重建 closure）。

## `Client()` 出站透传逻辑

```go
clientCh := pkgmwclient.NewClientChain(...).Append(jwt.Client())
```

每请求：

1. 从 jwt 私有 ctx 通道读 token：`tok, ok := TokenFromContext(ctx)`
2. ctx 缺 token / token 空 → passthrough（不报错）
3. 没有 client transport 附着 → passthrough（不报错）
4. 否则：`tr.RequestHeader().Set("Authorization", "Bearer "+tok)`

**业务必须显式 `Append(jwt.Client())`**——client 默认链不再自动添加。原因：不是每个出站调用都想透传入站凭据（跨 realm 调用、第三方集成、降权调用等）。这是从历史 `transport/client/middleware/chain.go` 自动 append 行为收回的。

## 私有 ctx 通道（context.go）

```go
type tokenKey struct{}                                  // 包私有
func WithToken(ctx, token) context.Context              // 写入
func TokenFromContext(ctx) (string, bool)               // 读取
```

`tokenKey` 是包私有结构体；`WithToken / TokenFromContext` 函数虽**导出**但语义只服务于：

- jwt 子包内部（`Server` 写 / `Authenticate` 读 / `Client` 读）
- 上游显式重新注入场景（如 retry / 网关层 token 翻译）

**其他引擎子包应当提供自己的 ctx 通道**：

- `mtls.WithCert(ctx, cert) / mtls.CertFromContext(ctx)`
- `apikey.WithKey(ctx, key) / apikey.KeyFromContext(ctx)`

通用 transport middleware 包（`transport/server/middleware/`、`transport/client/middleware/`）**禁止**承载这类引擎特化的 ctx helper——载体形态是引擎关切，不是框架关切。

## 包私有 `extractBearerToken`

```go
func extractBearerToken(header string) string   // 不导出
```

Bearer 解析逻辑（RFC 6750 风格）：

- 空 header → `""`
- 大小写不敏感 scheme（`Bearer` / `bearer` / `BEARER` 都接受）
- 多余空白 trim（`"Bearer  abc"` → `"abc"`）
- whitespace-only 或只有 scheme 的输入 → `""`
- 错误 scheme（如 `Basic xyz`）→ `""`
- 缺 scheme prefix → `""`

业务代码**永远不该**直接调这个函数——这是从历史公开 API `authn.ExtractBearerToken` 收回的。原因：`Bearer` 是 jwt-shaped 概念（mTLS 读 peer cert、API-Key 读自定义 header），框架主层不该有它。

## method 字符串：包私有常量

```go
const methodName = "jwt"   // 包私有
```

这个常量是 `*auditpb.AuthnDetail.Method` 字段在 jwt 路径下的 source of truth，传给 `authn.WithMethod(methodName)`。

**框架不枚举 authn 类型**——其他引擎子包用自己的 `methodName`（"mtls" / "apikey" / "passkey" / 业务自定义任意值）。proto 层 `servora.authn.v1.AuthnRule.schemes` 字段也是自由文本，跟 method 字符串一一对应。

## 典型业务用法

```go
import (
    authjwt "github.com/Servora-Kit/servora/security/authn/jwt"
    pkgmw "github.com/Servora-Kit/servora/transport/server/middleware"
    pkgmwclient "github.com/Servora-Kit/servora/transport/client/middleware"
)

// Server 端：完整 wrapper（标准 OIDC claims）
chain := pkgmw.NewServerChain(...).
    WithAudit(rec).
    Append(authjwt.Server(authjwt.WithVerifier(km.Verifier())))

// Server 端：Keycloak 特有 claims 映射
chain := pkgmw.NewServerChain(...).
    WithAudit(rec).
    Append(authjwt.Server(
        authjwt.WithVerifier(km.Verifier()),
        authjwt.WithClaimsMapper(authjwt.KeycloakClaimsMapper()),
    ))

// Client 端：显式出站透传 jwt token
clientCh := pkgmwclient.NewClientChain(...).
    Append(authjwt.Client())

// 高级：自组装 wrapper（如 token 不来自 HTTP header 而来自 message envelope）
auth := authjwt.NewAuthenticator(authjwt.WithVerifier(v))
mw := authn.Server(auth, authn.WithMethod("jwt"))
// ↑ 自己实现 ctx-write 的部分（决定怎么把 token 喂给 jwt.WithToken）
```

## Authenticate 行为

```go
func (a *authenticator) Authenticate(ctx context.Context) (actor.Actor, error)
```

- ctx 没 token 或 token 为空 → 返回 `actor.NewAnonymousActor(), nil`（pass-through）
- 没配 Verifier → 返回匿名（pass-through 模式，便于本地 / 测试）
- Verifier 校验失败 → 返回 `nil, err`（错误向上抛给 `authn.Server` dispatcher，进入失败 audit 路径）
- 校验成功 → 调 `claimsMapper(claims)` 返回 actor

## ClaimsMapper

- `DefaultClaimsMapper`：仅映射标准 OIDC claims（`sub`, `name`, `email`, `azp`, `scope`, `roles` / `role`）；**不**含 IdP 特有字段
- `KeycloakClaimsMapper`：在 Default 基础上额外映射
  - `iss → actor.UserActor.Realm`
  - `realm_access.roles → roles`（去重合并）

业务自定义 claims 解释：传 `authjwt.WithClaimsMapper(myMapper)`。

## 测试覆盖

```bash
go test ./security/authn/jwt/...
```

- **`extract_test.go`**：9 case 表驱动覆盖 `extractBearerToken` 边界
  - empty-header / canonical / lowercase / uppercase / double-space / scheme-only / wrong-scheme / no-scheme
- **`jwt_test.go`** 涵盖：
  - `TestServer_ExtractsBearerAndDispatches` — wrapper 完整 happy path（提 Bearer + 写 ctx + 委派 + AuthnDetail.Method = "jwt"）
  - `TestServer_ChainShortCircuit_PassthroughOnExistingActor` — 多机制 chain 短路（前置 actor 在则跳过引擎且不写 AuthnDetail）
  - `TestServer_NoBearerHeader_ReachesAuthenticator` — 缺 Authorization header 时仍委派 dispatcher，引擎走匿名路径
  - `TestClient_PropagatesToken` — 4 case：with-token / no-token-in-ctx / empty-token / no-client-transport
  - `TestAuthenticate_NoTokenInContext_ReturnsAnonymous` / `TestAuthenticate_NoVerifier_ReturnsAnonymous` — 引擎层匿名路径

## 边界约束

- 本包是 **JWT 引擎 wrapper**，不是 JWT 基础库；签发 / 验签的密码学细节在 `security/jwt`
- 不做业务 claims 解释规则（租户、项目等）—— 给业务通过 `WithClaimsMapper` 注入
- 不做资源级授权 —— 授权在 `security/authz`
- 不 import `transport/server/middleware/` 或 `transport/client/middleware/` 的引擎特化 helpers（这些都已删除；如果你看到这种 import 是历史残留）
- 不 import 父包 `security/authn` 的内部实现细节，只 import 公开接口（`authn.Authenticator`、`authn.Server`、`authn.WithMethod`）

## 常见反模式

- 在 `transport/server/middleware/` 下重新引入 `NewTokenContext / TokenFromContext` —— 这是 jwt 引擎私有概念，不是 transport 通用概念
- 在 `transport/client/middleware/chain.go` 默认 chain 里自动 append `jwt.Client()` —— 业务必须显式 opt-in
- 在业务代码里直接调 `extractBearerToken`（不可访问）或重新实现 Bearer 解析 —— 用 `jwt.Server(...)` wrapper
- 让其他引擎子包共享 `tokenKey{}` 或 `WithToken / TokenFromContext` —— 每个引擎应有自己的 ctx 通道
- 把 method 字符串改成 const 导出 —— 框架不枚举 authn 类型，业务自定义引擎自由命名

## 维护提示

- 修改 wrapper 三步顺序需同步更新 `jwt_test.go` 的对应断言（短路、AuthnDetail 写入位置、token 注入时机）
- 新增 Option 时同步在 `options.go` 加 With- 函数 + `authenticatorConfig` 字段 + 父包 `authn` 不需要任何改动
- 修 `extractBearerToken` 边界行为务必同步 `extract_test.go` 的 9 case
- `methodName` 常量值（`"jwt"`）跟 proto 层 `servora.authn.v1.AuthnRule.schemes` 自由文本对应；改名前确认下游审计 / 策略消费方的 schemes 配置
