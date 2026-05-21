# AGENTS.md - security/authn/jwt/

<!-- Parent: ../AGENTS.md -->
<!-- Updated: 2026-05-21 -->

## 子包定位

`security/authn/jwt` 是 Bearer JWT 认证 engine：读取 raw token、调用 `security/jwt.Verifier` 验签、用 `ClaimsMapper` 把 claims 写入 enriched context。

本包不是 JWT 密码学基础库；签发/验签实现归 `security/jwt`。本包也不绑定 Keycloak/Auth0/Okta/Cognito 等 IdP，富 claims 解释归业务 `ClaimsMapper`。

## 公开 API

```go
const Scheme = "jwt"

func Server(opts ...Option) middleware.Middleware
func Client() middleware.Middleware
func NewAuthenticator(opts ...Option) authn.Authenticator

func WithToken(ctx context.Context, token string) context.Context
func TokenFrom(ctx context.Context) (string, bool)
func WithClaims(ctx context.Context, claims gojwt.MapClaims) context.Context
func ClaimsFrom(ctx context.Context) (gojwt.MapClaims, bool)
func SubjectFrom(ctx context.Context) (string, bool)

func WithVerifier(v *security/jwt.Verifier) Option
func WithClaimsMapper(m ClaimsMapper) Option
type ClaimsMapper func(ctx context.Context, claims gojwt.MapClaims) (context.Context, error)
func DefaultClaimsMapper() ClaimsMapper
```

`Server(opts...)` 是单 JWT engine 便利包装，等价于 `authn.Server(authn.Multi(authn.Named(Scheme, NewAuthenticator(opts...))))`，并在 dispatch 前从入站 `Authorization: Bearer <token>` 写入本包私有 token ctx。

多 engine 场景不要叠 `jwt.Server`；直接在父包用 `authn.Server(authn.Multi(authn.Named(jwt.Scheme, jwt.NewAuthenticator(...)), ...))`。

## Token 与 claims 语义

- `Authenticate` 先读 `TokenFrom(ctx)`，再 fallback 到 Kratos server transport 的 `Authorization` header。
- 两处都没有 token：原 ctx + nil error passthrough。
- 未配置 verifier：原 ctx + nil error passthrough，便于本地/测试装配。
- 验签失败：返回带 `jwt: verify token` 上下文的 error。
- 验签成功后调用 `ClaimsMapper(ctx, claims)`，再写入 `authn.WithAuthType(enriched, "user")`。
- `DefaultClaimsMapper` 要求 `sub` 非空，并仅通过 `WithClaims` 保存完整 claims；不创建 actor。
- `SubjectFrom` 从 claims ctx 读取字符串 `sub`。

不要在默认 mapper 中加入 roles、email、groups、tenant、azp、scope 等 IdP/业务字段；业务需要时安装自定义 mapper。

## Client 透传

`Client()` 只把 `TokenFrom(ctx)` 中的 token 写到出站 `Authorization: Bearer ...`。缺 token、空 token 或无 client transport 时 passthrough。

默认 client middleware chain 不自动追加 `jwt.Client()`；调用方必须显式 `append(ms, jwt.Client())`，避免跨 realm、第三方集成或降权调用误透传凭据。

## 边界约束

- `extractBearerToken` 是包私 helper；业务不要依赖历史公开解析函数。
- `Scheme` 值与 proto `servora.authn.v1.AuthnRule.schemes` 自由文本对齐；改名需要全链路迁移。
- 其他 engine 不共享本包 token/claims ctx key，应定义自己的 ctx helper。
- 本包只 import 父包公开接口，不依赖父包内部 allowed/scheme holder。
- 不在本包实现资源授权；授权归 `security/authz`。

## 常见反模式

- 在 transport middleware 默认链里自动追加 `jwt.Client()`。
- 业务多引擎时用 `jwt.Server` 再叠 `apikey`/`mtls`。
- 把 IdP 特定 claims 加进 `DefaultClaimsMapper`。
- 把 `Scheme` 从 `"jwt"` 改为 `"bearer"`。

## 测试

```bash
GOWORK=off go test ./security/authn/jwt
```

关键覆盖：Bearer 解析边界、server wrapper 写 token、transport header fallback、ctx 优先级、client 透传、匿名路径、verifier 错误、默认 mapper、custom mapper、scheme 常量。
