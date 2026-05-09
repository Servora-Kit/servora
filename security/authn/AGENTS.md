# AGENTS.md - security/authn/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-22 | Updated: 2026-05-09 -->

## 模块定位

`security/authn` 是**引擎无关（engine-agnostic）**的认证调度器。它本身不持有任何凭据载体语义——不读 transport header、不解析 Bearer token、不写引擎私有的 ctx 通道。所有"从请求里取出凭据"的工作都委派给各引擎子包（`security/authn/jwt`、未来的 `security/authn/mtls` / `security/authn/apikey` 等）按对称模板自治。

主包只做三件事：

1. 调用 `Authenticator.Authenticate(ctx)` 拿到 `actor.Actor`
2. 把每次结果（成功 / 失败）以 `*auditpb.AuthnDetail` 形式 push 到 ctx，供末端 `audit.Collector` 单点 emit
3. 成功时把 actor 注入 ctx，调用下游 handler

设计意图详见 [`../../openspec/changes/authn-engine-agnostic-refactor/design.md`](../../openspec/changes/authn-engine-agnostic-refactor/design.md)。

## 目录结构

```
security/authn/
  authn.go          → Authenticator 接口（单方法）+ Server() 调度器 + Option（WithMethod / WithErrorHandler）
  authn_test.go     → 调度器层测试（fakeAuthenticator + 多种结果路径）
  jwt/              → jwt 引擎子包（自治 wrapper 模板，详见 jwt/AGENTS.md）
  noop/             → noop 引擎子包（永远匿名 actor）
```

## 核心接口（Authenticator）

```go
type Authenticator interface {
    Authenticate(ctx context.Context) (actor.Actor, error)
}
```

**接口契约（防滑坡承诺）**：仅容纳行为本体 `Authenticate`。**不**容纳：

- 引擎元数据（如历史的 `Method() string`）—— 改由 wrapper 通过 `authn.WithMethod(...)` option 在装配期传入；主包对字符串内容无感
- 编排钩子（`OnSuccess` / `Refresh` / `Tracer`）—— 调用方 / 容器 / sibling interface 责任
- 注入（logger / tracer）—— 容器责任
- infra 探测（`Health`）—— 单独的 sibling interface

这是接口的天花板，不是滑坡起点；未来谁想加新方法凭这条直接驳回。

**单方法形态的好处**：新引擎（mTLS、API-Key、AK+SK、Passkey 等）按 wrapper 模板加入时，引擎实现自由从 ctx 取凭据——jwt 从 jwt 私有 ctx 通道读、mtls 从 peer ctx 取 cert、api-key 从自己的 header 取——主包完全不需要知道凭据形态。

## 主包公开 API

```go
// Server 返回 Kratos 中间件。它是纯调度器：不读 transport，不写引擎私有 ctx 通道。
func Server(auth Authenticator, opts ...Option) middleware.Middleware

// WithMethod 设置 *auditpb.AuthnDetail.Method 字段。
// wrapper 子包必填（用包私有常量），业务直接调 Server 时也必填。
// 主包对字符串内容不做约束；缺省允许但不推荐。
func WithMethod(m string) Option

// WithErrorHandler 在 Authenticate 失败时被调用，用于把内部错误转成对外协议错误。
func WithErrorHandler(h func(ctx context.Context, err error) error) Option
```

## Audit ctx 写入路径

`Server()` 在以下两个结果点都会调用 `audit.WithAuthnResult(ctx, &auditpb.AuthnDetail{...})`：

| 场景                              | `Success` | `FailureReason` |
| --------------------------------- | --------- | --------------- |
| `Authenticator.Authenticate` 成功 | `true`    | `""`            |
| `Authenticator.Authenticate` 失败 | `false`   | `err.Error()`   |

**写入发生在返回 / 调用 errorHandler 之前**——authn 失败短路 return 时，ctx 已有 detail，外层 `audit.Collector` LIFO 后置阶段读到并 emit `AUTHN_RESULT` 事件。

`AuthnDetail.Method` 字段在装配期固定（`Server` 构造时缓存 `cfg.method`），不每请求 dispatch；调用方通过 `WithMethod(...)` 显式传入。

## Wrapper 子包对称模板（jwt 落地）

每个引擎子包按下面的四件套自治：

```
security/authn/<engine>/
  server.go       → Server(opts...) middleware.Middleware     ← 业务首选 entry
  client.go       → Client() middleware.Middleware             ← 出站凭证透传（如适用）
  context.go      → WithXxx(ctx, val) / XxxFromContext(ctx)    ← 引擎私有 ctx 通道
  <engine>.go     → NewAuthenticator(opts...) authn.Authenticator + Authenticate 实现
```

`Server(opts...)` wrapper 内部完成三步：

1. **chain 短路**：`if a, ok := actor.FromContext(ctx); ok && a.Type() != actor.TypeAnonymous { return handler(ctx, req) }`——零成本支撑 P0-4b 多机制 chain 组合（前一个引擎已成功认证则跳过）
2. **从 transport 提凭证**：写引擎私有 ctx 通道（如 jwt 的 `WithToken`）
3. **委派调度**：调 `authn.Server(auth, authn.WithMethod(<engine-private-method-string>))`

method 字符串是子包**私有常量**（如 jwt 的 `const methodName = "jwt"`），框架不枚举 authn 类型；业务自定义引擎可填任意字符串。

## 业务侧典型用法

```go
import (
    "github.com/Servora-Kit/servora/security/authn"
    authjwt "github.com/Servora-Kit/servora/security/authn/jwt"
    pkgmw "github.com/Servora-Kit/servora/transport/server/middleware"
)

// 80% 用例：直接 Append jwt 子包 wrapper
chain := pkgmw.NewServerChain(...).
    WithAudit(rec).
    Append(authjwt.Server(
        authjwt.WithVerifier(km.Verifier()),
        authjwt.WithClaimsMapper(authjwt.KeycloakClaimsMapper()),
    ))

// 出站透传 jwt token（client 默认链不再自动 append，必须显式）
clientCh := pkgmwclient.NewClientChain(...).
    Append(authjwt.Client())

// 高级：自实现 Authenticator + 直接调主包（业务自写 wrapper 也走同一路径）
chain := pkgmw.NewServerChain(...).
    Append(authn.Server(myCustomAuth, authn.WithMethod("custom")))
```

审计事件由 transport 链外层的 `audit.Collector(recorder)` 自动 emit——authn 主包只负责 ctx 写 detail，业务侧**无需**额外接线。详见 [`../../obs/audit/AGENTS.md`](../../obs/audit/AGENTS.md) 的 mounting 位置。

## 业务自定义机制

业务可在自己仓库写 `mybiz.PasskeyServer(...)` / `mybiz.PasskeyClient()`，走完全相同的对称模板：

- 仿照 `security/authn/jwt/{server,client,context,jwt}.go` 四件套结构
- 自己的引擎私有 ctx 通道（如 `mybiz.WithPasskey / mybiz.PasskeyFromContext`）
- 自己的 method 字符串常量（如 `const methodName = "passkey"`），传给 `authn.WithMethod(...)`
- 框架不知不见——proto 层 `servora.authn.v1.AuthnRule.schemes` 字段也是自由文本

## noop 子包

```go
// 用法：业务想显式注入匿名 actor 走完 audit pipeline 时
mw = append(mw, authn.Server(noop.NewAuthenticator(), authn.WithMethod("noop")))
```

- `noop.NewAuthenticator()` 返回总是产生匿名 actor 的 `Authenticator`（`Authenticate` 永远返回 `actor.NewAnonymousActor(), nil`）
- **不**提供 `noop.Server()` wrapper——noop 引擎没有载体 IO，wrapper 是空套子，不值当
- 构造器名跟 jwt 子包对齐（`NewAuthenticator`），调用方切换引擎不用重新学命名习惯

## 删除的公开 API（迁移指引）

| 旧 API（已删）                                                  | 新做法                                                                                                                                       |
| --------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- |
| `Authenticator.Method() string`（接口方法）                     | 改用 `authn.WithMethod("xxx")` option；wrapper 用包私有常量传入                                                                              |
| `authn.ExtractBearerToken(string) string`                       | 收回为 `security/authn/jwt` 包私有 `extractBearerToken`；业务永远不该直接调，用 `jwt.Server(...)` 即可                                       |
| `noop.New() *NoopAuthenticator`                                 | 重命名为 `noop.NewAuthenticator()` 并返回 `authn.Authenticator` 接口                                                                         |
| `transport/server/middleware.NewTokenContext / TokenFromContext` | 删除；jwt 子包自带 `jwt.WithToken / jwt.TokenFromContext` 引擎私有通道，其他引擎应当提供自己的（如 `mtls.WithCert / mtls.CertFromContext`）  |
| `transport/client/middleware.TokenPropagation`                  | 删除；用 `authjwt.Client()` 代替                                                                                                              |
| `transport/client/middleware/chain.go` 自动 append jwt 透传     | 删除；business 必须显式 `Append(authjwt.Client())`，因为不是每个出站调用都想透传入站凭证（跨 realm、第三方集成等）                            |

## 依赖方向

`security/authn` 主包仅 import：

- `core/actor`（actor 类型）
- `api/gen/go/servora/audit/v1`（中立 schema）
- `obs/audit`（仅 ctx helper `WithAuthnResult`，**不** import emitter / recorder 实现）
- `github.com/go-kratos/kratos/v2/middleware`（中间件 interface）

主包**不** import 任何 transport 包、不 import jwt-shaped 凭证解析逻辑。authn 无法感知最终事件落到哪里——这是 push-ctx + 末端 collector 单点 emit 的核心设计。

## 边界约束

- 本包是调度器层，不是 JWT 基础库；签发 / 验签细节在 `security/jwt`
- 新增认证引擎 = 新建 `security/authn/<engine>/` 子目录 + 实现四件套 wrapper 模板
- 本包不承载业务 claims 解释规则（租户、项目等）
- 本包不做资源级授权；授权决策在 `security/authz`
- 不在主包 import emit 实现，仅 import `audit.WithAuthnResult` ctx helper

## 常见反模式

- 在 `Authenticator` 接口上加新方法（hook / callback / probe / 元数据）—— 违反 contract 注释，直接驳回
- 在 `security/authn` 主包堆积业务 claims 解释和领域规则
- 把匿名身份、缺 token、验签失败三种状态混成一种处理
- 主包硬编码 `Method:"jwt"` 或类似引擎字符串—— 应由 wrapper 通过 `WithMethod(...)` 传入
- 在 `transport/{server,client}/middleware/` 下新增引擎特化的 token / cert / api-key helpers—— 这些都是引擎私有 ctx 通道，应该回到 `security/authn/<engine>/`
- 业务代码绕过 `actor` / wrapper，直接在业务层重复解析 token

## authn 注解 schemes 字段（v0.5.x 起）

`servora.authn.v1.AuthnRule.schemes`（与 `AuthnServiceDefault.schemes`）字段已在 proto schema 中落地，`protoc-gen-servora-authn` 也会把它生成到 `MethodSchemes()` 表中——**但运行时尚未消费**。当前 `security/authn.Server()` 仍是"单 Authenticator 实例 + wrapper 装配"模型：装配期注入哪些 wrapper，请求期就跑哪些；schemes 字段仅作为静态元数据存在于生成产物里。

后续多机制派发（按方法级 `schemes` 在 chain 内动态选择 / 串联多个 wrapper）跟踪在 `docs/plans/TODO.md` 的 **[P0-4b]**。本期重构（P0-6）已经为 P0-4b 留好了三个口子：

- 接口单方法 → 多个 wrapper 可同串一条链
- wrapper 一致带 chain 短路 → 前一个成功的 wrapper 不会被覆盖
- method 字符串走 `WithMethod(...)` 而非接口自描述 → 多 wrapper 串联时每个 wrapper 都填对自己的 method

设计意图（写下来防忘）：

- 接口层（`Authenticator`）保持引擎无关，schemes 派发逻辑应落在 chain 装配层，而非塞进单个 engine
- 派发表的来源是 plugin 生成的 `MethodSchemes()`（已就位），调用方在装配期把"scheme 名 → wrapper 实例"映射注入 chain
- mTLS / api-key 等新引擎到位前，schemes 字段在生产 proto 中应当填，但运行时如果只配了 jwt wrapper 也不会报错

## 测试与使用

```bash
go test ./security/authn/...
```

主包测试覆盖：成功 / 失败 / errorHandler 路径、AuthnDetail.Method 由 `WithMethod` 注入、缺省 method 允许。jwt 子包测试覆盖详见 `security/authn/jwt/AGENTS.md`。

## 维护提示

- 若调整 `ClaimsMapper` 字段，需同步检查 `core/actor` 接口契约（jwt 子包责任）
- 若新增认证引擎，在 `security/authn/<engine>/` 建子目录，按四件套模板实现；**不**要碰 `Authenticator` 接口
- 若 `auditpb.AuthnDetail` 字段调整，先改 proto + `make gen`，本包 ctx 写入处随 schema 自动更新
- 跨包 `replace` 仅本地 `go.work` 生效；版本号同步详见顶层 `AGENTS.md`
