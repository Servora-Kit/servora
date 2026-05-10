# AGENTS.md - security/authn/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-22 | Updated: 2026-05-10 -->

## 模块定位

`security/authn` 是**引擎无关（engine-agnostic）**的认证调度器。它本身不持有任何凭据载体语义——不读 transport header、不解析 Bearer token、不写引擎私有的 ctx 通道。所有"从请求里取出凭据"的工作都委派给各引擎子包（`security/authn/jwt`、`security/authn/apikey`、未来的 `security/authn/mtls` 等）按对称模板自治。

主包做四件事（P0-4b 落地的对齐 `authz.Server` 形态）：

1. 入口 chain 短路：检 `actor.From(ctx)` 已有非匿名 actor → passthrough（业务自家中间件已注入 actor 时兼容）
2. 注解表 `Rules` 路由：PublicMethods 命中 → 直接 passthrough；MethodSchemes 命中 → 装 `allowedSchemes` 包私 ctx 信道；未注解方法 → fail-open（allowed=nil）
3. 装 mutable `schemeHolder` 包私 ctx 信道，调 `Authenticator.Authenticate(ctx)` 拿 `actor.Actor`；`Multi` 在子引擎成功时反向写 holder 告诉主包"我成功的 scheme 是这个"
4. 把每次结果（成功 / 失败 / 多 engine 全失败聚合）以 `*auditpb.AuthnDetail` 形式 push 到 ctx，供末端 `audit.Collector` 单点 emit；成功时把 actor 注入 ctx，调用下游 handler

## 目录结构

```
security/authn/
  authn.go        → Authenticator 接口（单方法）+ Server 调度器 + Rules 类型 + Option（WithRulesFuncs / WithErrorHandler）
  multi.go        → Multi 装饰器 + Named + NamedAuthenticator + SchemeAttempt + SchemeAttemptsErr 公开 interface + schemeAttemptsErr 包私
  context.go      → 包私 allowedSchemes ctx 信道 + 包私 successfulScheme mutable holder ctx 信道
  authn_test.go   → 调度器层 + Multi 装饰器层测试
  jwt/            → jwt 引擎子包（自治 wrapper 模板，详见 jwt/AGENTS.md）
  noop/           → noop 引擎子包（永远匿名 actor）
  apikey/         → API-Key 引擎子包（in-memory Store stub，详见 apikey/AGENTS.md）
```

## 核心接口（Authenticator）

```go
type Authenticator interface {
    Authenticate(ctx context.Context) (actor.Actor, error)
}
```

**接口契约（防滑坡承诺）**：仅容纳行为本体 `Authenticate`。**不**容纳：

- 引擎元数据（如历史的 `Method() string`）—— 改由 `authn.Named(scheme, a)` 在装配期标注；主包对 scheme 字符串内容无感
- 编排钩子（`OnSuccess` / `Refresh` / `Tracer`）—— 调用方 / 容器 / sibling interface 责任
- 注入（logger / tracer）—— 容器责任
- infra 探测（`Health`）—— 单独的 sibling interface

这是接口的天花板，不是滑坡起点；未来谁想加新方法凭这条直接驳回。

**单方法形态的好处**：新引擎（mTLS、API-Key、AK+SK、Passkey 等）按 wrapper 模板加入时，引擎实现自由从 ctx 取凭据——jwt 从 jwt 私有 ctx 通道读、mtls 从 peer ctx 取 cert、api-key 从自己的 header 取——主包完全不需要知道凭据形态。

## 主包公开 API

```go
// 调度器中间件。单 engine 或 Multi(Named...) 装饰器都可作 a 传入。
func Server(a Authenticator, opts ...Option) middleware.Middleware

// 注解表聚合类型（与 authz.AuthzRule 对称）。
type Rules struct {
    PublicMethods []string             // MODE_PUBLIC RPC 路径列表
    MethodSchemes map[string][]string  // MODE_REQUIRED RPC 路径 → scheme 列表
}

// variadic 注入多模块 Rules；nil fn 跳过；MethodSchemes 后者覆盖前者。
func WithRulesFuncs(fns ...func() Rules) Option

// 业务自定义错误转换扩展点；错误已 detail 写过 ctx 后才进入 handler。
func WithErrorHandler(h func(ctx context.Context, err error) error) Option

// Multi 装饰器：first-success-wins 多 engine 串联。
type NamedAuthenticator struct { /* 包私字段 */ }
func Named(scheme string, a Authenticator) NamedAuthenticator
func Multi(named ...NamedAuthenticator) Authenticator

// 多 engine 全失败时返回的公开 error 形态。
type SchemeAttempt struct { Scheme, Reason string }
type SchemeAttemptsErr interface {
    error
    SchemeAttempts() []SchemeAttempt
}
```

`authn.WithMethod(string)` option **已删除**（method 字符串现在由 `Multi.Named` 在装配期推断；单 engine 直挂业务统一走 `Multi(Named(...))` 包装路径）。

## 调度器执行步骤（authn.Server 内部）

每个入站请求按下列顺序：

1. **chain 短路**：`actor.From(ctx)` 已存在非匿名 actor → 直接调 handler；不调 engine、不写 AuthnDetail
2. **operation 解析**：`transport.FromServerContext(ctx).Operation()` 拿当前 RPC 路径；非 server ctx 时 op="" 走全 fail-open
3. **PublicMethods passthrough**：op ∈ `Rules.PublicMethods` → 直接调 handler；不调 engine、不写 AuthnDetail（业务 handler 看到 ctx 不含 actor，按 anonymous 处理）
4. **allowed 集合构建**：op ∈ `Rules.MethodSchemes` → `allowed = set(MethodSchemes[op])`；未命中 → allowed=nil（fail-open）
5. **包私 ctx 信道装入**：`withAllowedSchemes` + `installSchemeHolder`
6. **dispatch**：调 `a.Authenticate(ctx)`
7. **成功**：从 holder 读 scheme 填 `AuthnDetail{Method:scheme, Success:true}`，注入 actor，调 handler
8. **失败**：
   - err 实现 `SchemeAttemptsErr` → `Method:"multi"`、`FailureReason: "scheme1: r1; scheme2: r2"` 聚合渲染
   - 其他 err → `Method:scheme`（来自 holder，可能为空）、`FailureReason: err.Error()`
   - 写 ctx detail 后调 `WithErrorHandler`（如配置）；否则返 `errors.Unauthorized("AUTHN_FAILED", reason)`

**写入发生在返回 / 调用 errorHandler 之前**——authn 失败短路 return 时，ctx 已有 detail，外层 `audit.Collector` LIFO 后置阶段读到并 emit `AUTHN_RESULT` 事件。

## Audit ctx 写入路径

| 场景                                                              | `Method`                  | `Success` | `FailureReason`                                |
| ----------------------------------------------------------------- | ------------------------- | --------- | ---------------------------------------------- |
| chain 短路（已有非匿名 actor）                                    | （未写）                  | （未写）  | （未写）                                       |
| PublicMethods 命中                                                | （未写）                  | （未写）  | （未写）                                       |
| 成功 + 走 Multi                                                   | 子引擎 scheme（如 "jwt"） | `true`    | `""`                                           |
| 成功 + 单 engine 直挂（不走 Multi）                               | `""`                      | `true`    | `""`                                           |
| 失败 + 单 engine                                                  | 同上 holder 内容（可能空）| `false`   | `err.Error()`                                  |
| 失败 + Multi 多 engine 全失败（err 实现 SchemeAttemptsErr）       | `"multi"`                 | `false`   | `"jwt: r1; apikey: r2"`（聚合）                |

## 包私 ctx 信道（不外泄）

| key 类型              | 装入                              | 读取                         | 用途                                    |
| --------------------- | --------------------------------- | ---------------------------- | --------------------------------------- |
| `allowedSchemesKey`   | `Server` 调 `Authenticate` 前     | `Multi.Authenticate` 内      | 注解表 schemes 集合传给 Multi 过滤      |
| `successfulSchemeKey` | `Server` 调 `Authenticate` 前     | `Server` 调 `Authenticate` 后 | mutable holder 接 Multi 反向写 scheme   |

两者均**包私**——`security/authn/` 目录外搜索 `allowedSchemesKey` / `successfulSchemeKey` / `schemeHolder` SHALL 找不到匹配。任何 engine 子包想自定义凭据传输应当起自己的 ctx 信道（如 `jwt.WithToken / jwt.TokenFrom`），不通过这两个内部 key。

## Multi 装饰器（标准用法）

```go
authn.Server(
    authn.Multi(
        authn.Named(jwt.Scheme,    jwt.NewAuthenticator(...)),
        authn.Named(apikey.Scheme, apikey.NewAuthenticator(...)),
    ),
    authn.WithRulesFuncs(
        examplev1.AuthnRules,
        iamv1.AuthnRules,
    ),
)
```

- 引擎按 `Named` **注入顺序**遍历（**不**按 allowed 顺序）
- `allowedSchemes != nil` → 仅参与 scheme 在集合中的子引擎；为 nil → 全集允许
- 第一个成功即收，后续子引擎 SHALL NOT 被调
- 全失败 → 聚合成 `*schemeAttemptsErr`（实现公开 `SchemeAttemptsErr` interface），主包识别后写 `Method:"multi"` 聚合渲染 `FailureReason`
- allowed 跟所有装的 engines 不交集 → 返 `errSchemesEmpty`（business proto 写 `schemes:["mtls"]` 但业务没装 mtls engine 时的兜底）

包装单 engine 场景**也应**走 `Multi(Named(...))`（业务统一形态）：

```go
authn.Server(
    authn.Multi(authn.Named(jwt.Scheme, jwt.NewAuthenticator(...))),
    authn.WithRulesFuncs(examplev1.AuthnRules),
)
```

## 业务侧典型用法

```go
import (
    "github.com/Servora-Kit/servora/security/authn"
    "github.com/Servora-Kit/servora/security/authn/jwt"
    "github.com/Servora-Kit/servora/security/authn/apikey"
    pkgmw "github.com/Servora-Kit/servora/transport/server/middleware"

    examplev1 "github.com/Servora-Kit/servora/api/gen/go/servora/example/v1"
)

ms := pkgmw.NewChainBuilder(httpLogger).
    WithAudit(rec).
    Build()

ms = append(ms, authn.Server(
    authn.Multi(
        authn.Named(jwt.Scheme,    jwt.NewAuthenticator(jwt.WithVerifier(km.Verifier()))),
        authn.Named(apikey.Scheme, apikey.NewAuthenticator(apikey.WithStore(store))),
    ),
    authn.WithRulesFuncs(examplev1.AuthnRules),
))
```

> **注意**：`pkgmw.ChainBuilder` 仅提供 `WithTrace / WithMetrics / WithAudit / WithoutRateLimit / Build` 等方法，**没有** fluent `Append`；业务侧用 Go 内建 `append(ms, mw...)` 追加 authn / authz / selector 等业务中间件。

审计事件由 transport 链外层的 `audit.Collector(recorder)` 自动 emit——authn 主包只负责 ctx 写 detail，业务侧**无需**额外接线。详见 [`../../obs/audit/AGENTS.md`](../../obs/audit/AGENTS.md) 的 mounting 位置。

## 业务自定义机制

业务可在自己仓库写 `mybiz.PasskeyAuthenticator()`，按引擎子包模板自治：

- 仿照 `security/authn/jwt/{server,client,context,jwt}.go` 四件套结构（如需 wrapper），或仅实现 `Authenticator` interface（power user）
- 自己的引擎私有 ctx 通道（如 `mybiz.WithPasskey / mybiz.PasskeyFrom`）
- 自己的 scheme 字符串常量（如 `const Scheme = "passkey"`），通过 `authn.Named(passkey.Scheme, ...)` 在 Multi 中标注
- 框架不知不见——proto 层 `servora.authn.v1.AuthnRule.schemes` 字段也是自由文本

## WithErrorHandler 拿到 SchemeAttempts

```go
authn.Server(
    authn.Multi(...),
    authn.WithErrorHandler(func(ctx context.Context, err error) error {
        if as, ok := err.(authn.SchemeAttemptsErr); ok {
            for _, a := range as.SchemeAttempts() {
                log.Errorw("authn attempt failed",
                    "scheme", a.Scheme, "reason", a.Reason)
            }
        }
        return err
    }),
    ...
)
```

`*schemeAttemptsErr` 包私 type 实现公开 `SchemeAttemptsErr` interface，外部只看到 interface；`SchemeAttempts()` 返回 attempts 副本，handler 不能 mutate 内部状态。

## 删除的公开 API（迁移指引）

| 旧 API（已删）                          | 新做法                                                                                                         |
| --------------------------------------- | -------------------------------------------------------------------------------------------------------------- |
| `Authenticator.Method() string`（接口方法） | 改用 `authn.Named(scheme, a)` 在装配期标注；Multi 反向写 holder                                                |
| `authn.WithMethod(string)` Option       | 删除；统一走 `authn.Multi(authn.Named(...))` 装饰器路径                                                        |

## 依赖方向

`security/authn` 主包仅 import：

- `core/actor`（actor 类型）
- `api/gen/go/servora/audit/v1`（中立 schema）
- `obs/audit`（仅 ctx helper `WithAuthnResult`，**不** import emitter / recorder 实现）
- `github.com/go-kratos/kratos/v2/{middleware,errors,transport}`（中间件 / errors / transport server ctx 抽取）

主包**不** import 任何 jwt-shaped 凭证解析逻辑。authn 无法感知最终事件落到哪里——这是 push-ctx + 末端 collector 单点 emit 的核心设计。

## 边界约束

- 本包是调度器层，不是 JWT 基础库；签发 / 验签细节在 `security/jwt`
- 新增认证引擎 = 新建 `security/authn/<engine>/` 子目录 + 实现 `Authenticator` interface + 暴露 `const Scheme = "<engine>"`
- 本包不承载业务 claims 解释规则（租户、项目等）；OIDC 字段不在 actor 三件套中，业务侧用自定义 ctx 信道传输
- 本包不做资源级授权；授权决策在 `security/authz`
- 不在主包 import emit 实现，仅 import `audit.WithAuthnResult` ctx helper

## 常见反模式

- 在 `Authenticator` 接口上加新方法（hook / callback / probe / 元数据）—— 违反 contract 注释，直接驳回
- 在 `security/authn` 主包堆积业务 claims 解释和领域规则
- 在 `Multi` 装饰器内部写 ctx AuthnDetail —— 由 `authn.Server` 单点写入，避免双写覆盖
- 引擎子包直接读 `allowedSchemesKey` / `successfulSchemeKey` —— 这两个 key 是 authn 主包内部协议，不向外暴露
- 业务代码绕过 `actor` / `Multi`，直接在业务层重复解析 token

## authn 注解 schemes 字段（v0.6.x 起运行时消费）

`servora.authn.v1.AuthnRule.schemes`（与 `AuthnServiceDefault.schemes`）由 `protoc-gen-servora-authn` 生成进 `AuthnRules() Rules` 聚合产物。`authn.Server` 在请求期：

1. 命中 `Rules.MethodSchemes` → 装 allowed 集合到 `allowedSchemes` ctx 信道
2. `Multi` 读 allowed 决定遍历过滤
3. allowed 跟装的 engines 不交集 → `Multi` 返 `errSchemesEmpty`，主包渲染成 401

**注意**：单 engine 直挂业务（不走 Multi）SHALL 仍兼容——`allowedSchemes` 装入但单 engine 不读，holder 不写，AuthnDetail.Method 留空。业务侧推荐统一走 `Multi(Named(...))`。

## 测试与使用

```bash
GOWORK=off go test ./security/authn
```

主包测试覆盖：chain 短路、PublicMethods passthrough、MethodSchemes allowed 装入、未注解 fail-open、单 engine 成功 / 失败 detail 写入、Multi 全失败聚合 detail（Method:"multi"）、`WithRulesFuncs` 多 fn merge + nil 跳过 + 后者覆盖、Multi first-success-wins、Multi allowed 过滤、Multi empty 交集→errSchemesEmpty、Multi 写 holder、Multi 注入顺序遍历、`SchemeAttemptsErr` interface 断言、外层 Collector 失败路径仍 emit。

## 维护提示

- 若 `auditpb.AuthnDetail` 字段调整，先改 proto + `make gen`，本包 ctx 写入处随 schema 自动更新
- 若新增认证引擎，在 `security/authn/<engine>/` 建子目录，实现 `Authenticator` interface 并暴露 `const Scheme = "<engine>"`；**不**要碰 `Authenticator` 接口
- 若调整 `Rules` 字段，需同步 `cmd/protoc-gen-servora-authn` plugin 输出 + `obs/audit` 写入路径（一般不需）
- 跨包 `replace` 仅本地 `go.work` 生效；版本号同步详见顶层 `AGENTS.md`
