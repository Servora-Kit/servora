# AGENTS.md - security/authn/

<!-- Parent: ../AGENTS.md -->
<!-- Updated: 2026-07-09 -->

## 模块定位

`security/authn` 是引擎无关的认证调度器。它不读取具体凭据、不解析 Bearer token、不写 engine 私有 ctx 通道；凭据形态由 `jwt/`、`apikey/`、`noop/` 或业务自定义 engine 自治。

主包只负责：

- 根据生成的 `map[string]*authnpb.AuthnRule` 区分 public RPC、required RPC 与未注解 fail-open RPC。
- 在调用 engine 前安装包私 `allowedSchemes`。
- 调用单方法 `Authenticator.Authenticate(ctx)`，接收 engine 返回的 enriched context。
- 认证失败时返回 401 或调用 `WithErrorHandler`。
- 配置了 `WithAuditor(audit.Auditor)` 时直接发 CloudEvents 成功/失败事件。

## 公开边界

```go
type Authenticator interface {
    Authenticate(ctx context.Context) (context.Context, error)
}

func Server(a Authenticator, opts ...Option) middleware.Middleware
func WithRulesFuncs(fns ...func() map[string]*authnpb.AuthnRule) Option
func WithErrorHandler(h func(context.Context, error) error) Option
func WithAuditor(a audit.Auditor) Option
func Named(scheme string, a Authenticator) NamedAuthenticator
func Multi(named ...NamedAuthenticator) Authenticator
var ErrNoCredentials error

func WithAuthType(ctx context.Context, authType string) context.Context
func AuthTypeFrom(ctx context.Context) (string, bool)
func SubjectFromAny(fns ...func(context.Context) (string, bool)) func(context.Context) (string, bool)
```

`Authenticator` 必须保持单方法。不要重新加入 `Method()`、hook、callback、probe、logger/tracer 注入或 health check。

## 执行语义

- `MODE_PUBLIC` RPC passthrough，不调用 engine。
- `MODE_REQUIRED` 且 schemes 非空：allowed schemes 进入包私 ctx，`Multi` 只遍历匹配 scheme。
- `MODE_REQUIRED` 且 schemes 为空：allowed=nil，使用已装认证 engine 的默认集合。
- 未注解 RPC：fail-open，allowed=nil，允许所有已装 engine 参与。
- `Multi` first-success-wins，按 `Named(...)` 注入顺序遍历，不按 proto schemes 顺序排序。
- engine 没看到自己的凭据时必须返回匹配 `ErrNoCredentials` 的错误，`Multi` 继续尝试后续 engine。
- engine 看到凭据但验证失败、后端失败或配置失败时返回普通错误，`Multi` fail-fast。
- allowed 与已装 engines 无交集时返回 `errSchemesEmpty`，上层渲染为 401。
- 多 engine 全无凭据时错误同时匹配 `ErrNoCredentials` 并实现 `SchemeAttemptsErr`，failure audit 使用聚合 reason。
- engine 成功时应在返回 ctx 中写入自己拥有的认证元数据，例如 auth type、subject 来源或 claims/key meta。

`WithAuditor` 使用 `Auditor.Emit(ctx, cloudevents.Event)` 直接发送事件：

- 认证失败路径：事件类型 `servora.authn.failure.v1`，data 使用 `authnauditpb.AuthnFailure` proto。
- 认证成功路径（`emitAuthnSuccess`）：事件类型 `servora.authn.success.v1`，data 使用 `authnauditpb.AuthnSuccess` proto。

`NewEvent()` 负责 CloudEvents required attributes 和服务级 source；authn 不再注入 severity。`SubjectFromAny` 与 `AuthTypeFrom` 仍然可用，但 audit middleware 本身不再调用它们；它们仅供其他用途（如 authz subject 提取）使用。这里没有旧版 runtime detail 或 context holder。

## 子包职责

- `jwt/`：Bearer JWT engine，负责 raw token 读取、验签、claims -> ctx enrichment 和可选出站透传。
- `apikey/`：API key engine，负责 `X-API-Key` 读取并委派业务 `Store.Lookup` 得到 `KeyMeta`。
- `noop/`：测试/占位 engine。

新增 engine 放在 `security/authn/<engine>/`，暴露 `const Scheme = "<engine>"` 并实现 `Authenticator`。若需要凭据透传，定义自己的 ctx helper，不要触碰主包的包私 keys。

## 依赖方向

主包可以依赖 Kratos middleware/errors/transport、`obs/audit` 的 `Auditor` 抽象和 CloudEvents envelope。
主包不要 import `security/authn/jwt`、`security/authn/apikey`、任何凭据解析逻辑、audit 后端实现或业务 claims 解释规则。

## 常见反模式

- 在主包解析 Authorization、X-API-Key、mTLS cert 或业务 token。
- 将未注解 RPC 改成 fail-closed，除非同步生成器与下游迁移策略。
- 业务多引擎时叠多个单 engine wrapper；应使用 `authn.Server(authn.Multi(authn.Named(...)))`。
- 把资源级授权塞进 authn；授权归 `security/authz`。
- 在 authn 中构造业务 actor 或业务权限模型。

## 测试

```bash
go test ./security/authn/...
```

关键覆盖：public passthrough、required schemes allowed 过滤、required 空 schemes 默认认证集合、未注解 fail-open、规则合并、Multi first-success-wins、空交集错误、`ErrNoCredentials`、`SchemeAttemptsErr`、成功/失败 CloudEvents audit。
