# AGENTS.md - security/authn/apikey/

<!-- Parent: ../AGENTS.md -->
<!-- Updated: 2026-05-24 -->

## 子包定位

`security/authn/apikey` 是 API key 认证 engine：读取 `X-API-Key`，把 key 交给业务注入的 `Store.Lookup(ctx, key)`，由 Store 返回最小 `KeyMeta`。

本包不提供 key 管理、签发、撤销、rotate、持久化或缓存实现。具体存储后端属于业务仓库。

## 公开 API

```go
const Scheme = "apikey"

type KeyMeta struct {
    KeyID   string
    OwnerID string
}

type Store interface {
    Lookup(ctx context.Context, key string) (KeyMeta, error)
}

func NewAuthenticator(opts ...Option) authn.Authenticator
func WithStore(s Store) Option
func WithKeyMeta(ctx context.Context, meta KeyMeta) context.Context
func KeyMetaFrom(ctx context.Context) (KeyMeta, bool)
func SubjectFrom(ctx context.Context) (string, bool)
```

`WithStore` 必传；`NewAuthenticator()` 缺 Store 时 panic `apikey: WithStore is required`。这是装配错误，必须启动期 fail-fast。

本包没有 `Server(opts...)` 单引擎 wrapper、没有 `Client()`、没有 raw key ctx 透传、没有 ClaimsMapper。多 engine 统一用父包 `authn.Server + authn.Multi`。

## 执行语义

- 无 server transport、缺 `X-API-Key`、header 空值：返回匹配 `authn.ErrNoCredentials` 且包含 `missing X-API-Key` 的错误。
- 有 key：逐字调用 `Store.Lookup(ctx, key)`。
- Lookup 成功：写入 `WithKeyMeta(ctx, meta)`，再写入 `authn.WithAuthType(ctx, "api_key")`，返回 enriched ctx。
- Lookup 失败：错误原样返回，`Multi` 视为无效凭据/后端失败并 fail-fast。
- `SubjectFrom` 读取 `KeyMeta.OwnerID`；`KeyID` 是 key 标识，不是 secret。
- 错误信息可能进入失败 audit CloudEvents，必须避免泄漏 key 原文或 PII。

典型组合：人类用户走 jwt，服务账号走 apikey。

```go
authn.Server(
    authn.Multi(
        authn.Named(jwt.Scheme, jwt.NewAuthenticator(jwt.WithVerifier(v))),
        authn.Named(apikey.Scheme, apikey.NewAuthenticator(apikey.WithStore(store))),
    ),
    authn.WithRulesFuncs(examplev1.AuthnRules),
)
```

## 边界约束

- Header 名固定为 `X-API-Key`；不要改到 Authorization，避免与 jwt engine 冲突。
- `Scheme` 值与 proto schemes 对齐；改名需要全链路迁移。
- 不在框架主仓新增 `NewMemoryStore`；测试 fixture 属于使用方仓库。
- 不在 Store 中做 path/resource 授权；授权归 `security/authz`。
- Store 只返回框架最小 `KeyMeta`；业务富数据放业务 ctx/helper。

## 常见反模式

- Store 对未知 key 返回空 `KeyMeta` + nil。
- 为了“方便测试”让缺 Store 静默接受。
- 在本包保存 raw key 到 ctx，导致后续链路误透传凭据。
- 把 API key 生命周期管理塞入框架 engine。

## 测试

```bash
GOWORK=off go test -race ./security/authn/apikey
```

关键覆盖：scheme 常量、缺 Store panic、缺 header、无 transport、happy path、Store 错误传播、`X-API-Key` 提取边界、KeyMeta/SubjectFrom。
