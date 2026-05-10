# AGENTS.md - security/authn/oauth2/

## 子包定位

OAuth 2.0 access token 验证业务的位置预留（位置标记，本期 v0.6.0 不实现）。

## 本期不实现

本子包仅含本 AGENTS.md，无 .go 文件，无测试，无 export。

- 业务用 OAuth 2.0 access token 认证：暂以 `jwt.NewAuthenticator` + 自定义 `ClaimsMapper` 替代
- IdP-specific 适配（Keycloak / Auth0 / Okta 等）：归 servora-platform 后期实现

## 未来实现边界

后期实现时本子包应提供：

- 消费 `github.com/Servora-Kit/servora/security/jwt.Verifier` 做底层 token 验签（不要重新发明轮子）
- 标准 OAuth 2.0 claims 映射 `ClaimsMapper`（`sub` / `azp` / `scope` / 等等），与 `jwt` 子包的极简 `DefaultClaimsMapper` 形成两层
- `oauth2.NewAuthenticator(opts ...Option) authn.Authenticator`
- `oauth2.Server(opts ...Option) middleware.Middleware`（power-user 单引擎便利包装）
- `oauth2.Client() middleware.Middleware`（出站透传）
- `oauth2.WithVerifier(...)` / `oauth2.WithClaimsMapper(...)` Option
- `const Scheme = "oauth2"`

集成方式（多引擎组合示例）：

```go
authn.Server(
    authn.Multi(
        authn.Named(jwt.Scheme,    jwt.NewAuthenticator(...)),
        authn.Named(oauth2.Scheme, oauth2.NewAuthenticator(oauth2.WithVerifier(jv))),
    ),
    authn.WithRulesFuncs(myProto.AuthnRules),
)
```

## servora-platform 平等位

servora-platform 也可独立实现等价子包（如 `servora-platform/iam/oauth2`），跟主仓 `oauth2` 子包平等。主仓子包定位为「vendor-neutral OAuth 2.0 通用骨架」，IdP-specific 扩展归 platform。

## 维护提示

- 添加任何 .go 文件前请同时更新本 AGENTS.md（删除「本期不实现」段）
- 实现时严格区分本子包与 OIDC 子包：access token 验签归 `oauth2`；`id_token` / `userinfo` / discovery 归 `oidc`
- 与 `jwt` 子包的边界：`jwt` 是通用 Bearer JWT 骨架（最小三件套 `ClaimsMapper`），`oauth2` 是其上的 OAuth 2.0 标准 claims 适配层
