# AGENTS.md - security/authn/oidc/

## 子包定位

OpenID Connect (OIDC) `id_token` / `userinfo` / discovery 业务的位置预留（位置标记，本期 v0.6.0 不实现）。

## 本期不实现

本子包仅含本 AGENTS.md，无 .go 文件，无测试，无 export。

- 业务用 OIDC 认证：暂以 `jwt` 子包 + 自定义 `ClaimsMapper` 替代
- Discovery / UserInfo 等 OIDC-specific 流程：业务侧自行实现或等本子包后期落地

## 未来实现边界

后期实现时本子包应提供：

- `id_token` 验签（复用 `github.com/Servora-Kit/servora/security/jwt.Verifier` 或独立路径）
- `oidc.UserInfoFromContext(ctx)` ctx accessor — 业务从 ctx 拿 OIDC 标准 userinfo 字段（`email` / `preferred_username` / `picture` / `locale` 等）
- UserInfo endpoint 客户端
- Discovery (`/.well-known/openid-configuration`) 客户端
- `oidc.NewAuthenticator(opts ...Option) authn.Authenticator`
- `oidc.Server(opts ...Option) middleware.Middleware`
- `oidc.Client() middleware.Middleware`
- `oidc.WithVerifier(...)` / `oidc.WithDiscoveryURL(...)` / `oidc.WithUserInfoEndpoint(...)` Option
- `const Scheme = "oidc"`

## 与 oauth2 子包的边界

OIDC ⊂ OAuth 2.0：

- 业务接 OAuth 2.0 access token → 用 `oauth2` 子包
- 业务接 OIDC `id_token` 验证 + UserInfo + Discovery → 用本（`oidc`）子包
- 简单 Bearer JWT（自家 IdP 签发，不走 OAuth 2.0 / OIDC 标准）→ 用 `jwt` 子包

## servora-platform 平等位

servora-platform 也可独立实现等价子包（如 `servora-platform/iam/oidc-keycloak`）。主仓 `oidc` 子包定位为「vendor-neutral OIDC 通用骨架」，IdP-specific 扩展（Keycloak realm 概念、Auth0 namespace 概念等）归 platform。

## 维护提示

- 添加任何 .go 文件前请同时更新本 AGENTS.md
- `id_token` 验签建议复用 `oauth2` 子包的 `Verifier` 抽象，避免双轨
- UserInfo 字段应通过包私 ctx 信道（如 `userInfoKey struct{}`）传递，accessor 命名遵循全仓约定 `From(ctx)` 不含 `Context`
