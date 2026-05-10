# AGENTS.md - security/authn/apikey/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-05-10 -->

## 子包定位

`security/authn/apikey` 是 **API key 认证骨架（stub）**——只承担「读 X-API-Key header + 委派 Store 查询」的最小骨架，不绑定任何存储后端（in-memory / DB / 缓存 / 跨服务 RPC 通通由业务在外层通过 `Store` 接口注入）。

跟 `security/authn/jwt` 对称的双引擎站位：

- **jwt**：消费 `Authorization: Bearer <token>`，通过 `Verifier + ClaimsMapper` 把 JWT claim 解释成 actor
- **apikey**：消费 `X-API-Key: <key>`，通过 `Store.Lookup(ctx, key)` 把字符串解释成 actor

两个引擎可以同时挂在 `authn.Multi` 下，因为 header 载体不冲突（一个吃 Authorization，一个吃 X-API-Key），互不干扰。

## 目录结构

```
security/authn/apikey/
  apikey.go         → 包注释 + Scheme 常量
  store.go          → Store 接口定义
  extract.go        → extractAPIKey（包私有，从 X-API-Key header 抽 key）
  options.go        → Option + WithStore
  authenticator.go  → NewAuthenticator + authenticator struct
  apikey_test.go    → 9 case 覆盖构造 / Authenticate / extract 全路径
```

## 公开 API

```go
// 包级常量（与 proto schemes / authn.Named 配对）
const Scheme = "apikey"

// 业务必须实现的存储后端接口
type Store interface {
    Lookup(ctx context.Context, key string) (actor.Actor, error)
}

// 构造引擎；至少一个 WithStore 必传，否则 panic
func NewAuthenticator(opts ...Option) authn.Authenticator

// Options
type Option func(*config)
func WithStore(s Store) Option
```

**没有** `Server(opts...)` 单引擎便利包装、**没有** `Client()` 出站透传、**没有** `WithToken / TokenFrom` ctx 通道、**没有** `ClaimsMapper` 扩展点——这些都是 jwt 引擎的特化关切，apikey 引擎用不到。

## Store 接口：业务自己实现

`Store.Lookup(ctx, key)` 是从字符串到 actor 的唯一桥梁。框架不规约存储形态，业务可以选：

### in-memory（灯塔 e2e / 单测）

```go
// servora-example/example/internal/stubapikey/store.go (灯塔仓库内, 非 servora 主仓)
package stubapikey

type Store struct {
    keys map[string]actor.Actor
}

func NewStore() *Store {
    return &Store{
        keys: map[string]actor.Actor{
            "valid-test-key": actor.NewServiceActor("test-svc", "Test Service"),
        },
    }
}

func (s *Store) Lookup(_ context.Context, key string) (actor.Actor, error) {
    a, ok := s.keys[key]
    if !ok { return nil, errors.New("apikey: unknown key") }
    return a, nil
}
```

### DB-backed

```go
type pgStore struct { db *sql.DB }

func (s *pgStore) Lookup(ctx context.Context, key string) (actor.Actor, error) {
    var id, name string
    err := s.db.QueryRowContext(ctx,
        "SELECT actor_id, display_name FROM api_keys WHERE key = $1 AND revoked_at IS NULL",
        key,
    ).Scan(&id, &name)
    if err != nil {
        return nil, fmt.Errorf("apikey: lookup: %w", err)
    }
    return actor.NewServiceActor(id, name), nil
}
```

### 缓存 / RPC

业务自己包一层即可，框架完全不感知。

> **注意**：`actor.Actor` 的具体类型（`*UserActor` / `*ServiceActor` / 自定义实现）由 `Store` 决定。人工签发的 key 通常映射成 `UserActor`，服务账号 key 映射成 `ServiceActor`。

## WithStore 是**必传**：fail-fast panic

```go
auth := apikey.NewAuthenticator()  // ❌ panic: "apikey: WithStore is required"
```

apikey 引擎没有 Store 时无法解释任何 key，所以「忘传」一定是装配 bug。fail-fast 让 bug 在启动时被发现，而不是每个请求 401。

```go
auth := apikey.NewAuthenticator(apikey.WithStore(myStore))  // ✅ 正确
```

## 多引擎组合（推荐生产姿态）

业务最常见的姿态是 jwt + apikey 双挂——人类用户走 jwt，服务账号走 apikey：

```go
import (
    "github.com/Servora-Kit/servora/security/authn"
    "github.com/Servora-Kit/servora/security/authn/jwt"
    "github.com/Servora-Kit/servora/security/authn/apikey"
)

mw = append(mw, authn.Server(
    authn.Multi(
        authn.Named(jwt.Scheme,    jwt.NewAuthenticator(jwt.WithVerifier(v))),
        authn.Named(apikey.Scheme, apikey.NewAuthenticator(apikey.WithStore(myStore))),
    ),
    authn.WithRulesFuncs(examplev1.AuthnRules),
))
```

`authn.Multi` first-success-wins：先试 jwt（找 Authorization header），失败则试 apikey（找 X-API-Key header），都失败合成 `schemeAttemptsErr` 让 dispatcher 写 `AuthnDetail{Method:"multi", FailureReason:"jwt: ...; apikey: ..."}`。

## 灯塔 e2e 装配

灯塔 in-memory `Store` 实现 **不在** servora 主仓本子包，而在 `servora-example/example/internal/stubapikey/`。理由：

- 主仓提供「骨架 + 接口」，业务提供「具体存储」
- 灯塔 e2e 拟用的 in-memory key 表是测试 fixture，不应进框架发布产物
- 业务想抄实现就 fork stubapikey 改改即可

灯塔装配示意（详见 servora-example）：

```go
authn.Server(
    authn.Multi(
        authn.Named(jwt.Scheme,    jwt.NewAuthenticator(jwt.WithVerifier(jv))),
        authn.Named(apikey.Scheme, apikey.NewAuthenticator(apikey.WithStore(stubapikey.NewStore()))),
    ),
    authn.WithRulesFuncs(examplev1.AuthnRules),
)
```

## Authenticate 行为

```go
func (a *authenticator) Authenticate(ctx context.Context) (actor.Actor, error)
```

- ctx 没 transport / X-API-Key header 缺失 / 值为空 → 返回 `(nil, errMissingHeader)`，错误信息含 `"missing X-API-Key"`
- 否则调 `Store.Lookup(ctx, key)` 并**逐字传递**结果（actor + err）

错误信息保持 PII-free：上层 `AuthnDetail.FailureReason` 会原样出现在审计日志里。

## 没有 ClaimsMapper 扩展点

apikey 没有 JWT claim 这个概念——actor 是 `Store` 直接构造的。如果业务想根据 key 元数据（颁发者 / 范围 / 过期时间等）富化 actor，应该在 `Store` 实现内部完成，不需要框架提供 mapper hook。

## `Scheme` 常量

```go
const Scheme = "apikey"
```

跟 `protoc-gen-servora-authn` 输出的 `AuthnRule.schemes` 自由文本一一对应。这个值通过 `authn.Named(apikey.Scheme, ...)` 喂给 `authn.Multi`，最终落到 `*auditpb.AuthnDetail.Method` 字段。

**框架不枚举 authn 类型**——其他引擎子包用自己的 `Scheme` 常量（`jwt.Scheme = "jwt"` / `mtls.Scheme = "mtls"` / 业务自定义任意值）。

## 测试覆盖

```bash
GOWORK=off go test -race ./security/authn/apikey
```

- `TestScheme_IsApikey` — 常量值哨兵
- `TestNewAuthenticator_WithoutStore_Panics` — 缺 WithStore 时 fail-fast panic
- `TestAuthenticate_MissingHeader_ReturnsError` — transport 在但 X-API-Key 缺 → 返 `missing X-API-Key`
- `TestAuthenticate_NoTransport_ReturnsError` — bare ctx 无 transport → 同样路径
- `TestAuthenticate_HappyPath` — 合法 key 经 Store 解析回 ServiceActor
- `TestAuthenticate_StoreError_Propagates` — Store 报错（撤销 key 等）逐字传递
- `TestExtractAPIKey_TransportPresent` — header 有值时正确读出
- `TestExtractAPIKey_NoTransport` — 无 transport 返 `""`
- `TestExtractAPIKey_HeaderAbsent` — transport 在但 X-API-Key 缺（Authorization 在也不算）→ `""`

## 边界约束

- 本包是 **API key 认证骨架**，不是 key 管理库；签发 / 撤销 / rotate 在业务的 `Store` 实现里
- 不做 Authorization Bearer 解析（那是 `security/authn/jwt` 的事）
- 不做密码学校验（API key 是不透明字符串，安全性靠存储后端 + TLS）
- 不做资源级授权（授权在 `security/authn` 的 dispatcher + `security/authz`）
- 不 import 父包 `security/authn` 的内部实现细节，只 import 公开接口（`authn.Authenticator`）
- 不在主仓提供具体 `Store` 实现（in-memory 测试 stub 归 servora-example）

## 常见反模式

- 把 X-API-Key 改成 Authorization header → 与 jwt 子包冲突，破坏多引擎共存能力
- 在主仓提供 `apikey.NewMemoryStore(...)` → 测试 fixture 应当下沉到使用方仓库
- 让 `Store.Lookup` 在 key 未知时返回 `(actor.NewAnonymousActor(), nil)` → 应当返 error，让 `Multi` 进入下一个 engine
- 在 `Store` 里偷偷做授权（按 path 拒绝）→ 授权归 `security/authz`，apikey 只负责回 actor
- 业务忘传 `WithStore` → fail-fast panic 兜底，**不要**改成 silently 接受
- 在子包里加 `WithKey / KeyFrom` ctx 通道 → apikey 不需要 raw key 流转（不像 jwt 需要把 token 透传到 client middleware）

## 维护提示

- 修改 header 名（`X-API-Key`）需同步更新 `extract.go` 注释 + AGENTS.md + 灯塔 e2e 测试
- 修改错误信息（`apikey: missing X-API-Key header`）务必保留 `"missing X-API-Key"` 子串——`apikey_test.go` 用 `strings.Contains` 断言
- 修改 `panic` 消息（`apikey: WithStore is required`）务必同步 `TestNewAuthenticator_WithoutStore_Panics` 的精确比对
- `Scheme` 常量值（`"apikey"`）跟 proto 层 `servora.authn.v1.AuthnRule.schemes` 自由文本对应；改名前确认下游审计 / 策略消费方的 schemes 配置
- 新增 `Option` 时同步在 `options.go` 加 `With-` 函数 + `config` 字段；父包 `authn` 不需要任何改动
- 想加 `ClaimsMapper`？**先停下**：apikey 没有 claim 概念，actor 富化属于 `Store` 实现职责
