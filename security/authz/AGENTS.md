# AGENTS.md - security/authz/

<!-- Parent: ../AGENTS.md -->
<!-- Updated: 2026-07-09 -->

## 模块定位

`security/authz` 是资源级授权 middleware。它消费 `protoc-gen-servora-authz` 生成的 `map[string]*authzpb.AuthzRule`，从 ctx/req 提取 subject 与 resource，调用 `Authorizer.Check`，并在 allowed/denied/error 三条路径可选发 CloudEvents audit。

本包不做身份认证、不解析 JWT/API key、不管理 OpenFGA tuple。具体引擎在 `openfga/` 等子包。

## 公开边界

```go
type Authorizer interface {
    Check(ctx context.Context, req CheckRequest) (bool, error)
}

type CheckRequest struct {
    Subject      string
    Action       string
    ResourceType string
    ResourceID   string
    Attributes   map[string]any
}

func Server(a Authorizer, opts ...Option) middleware.Middleware
func WithRulesFuncs(fns ...func() map[string]*authzpb.AuthzRule) Option
func WithDefaultResourceID(id string) Option
func WithCheckTimeout(d time.Duration) Option
func WithFailOpenOnMissingRule(alert func(context.Context, string)) Option
func WithSubjectFunc(fn func(context.Context) (string, bool)) Option
func WithAuditor(a audit.Auditor) Option
```

`WithFailOpenOnMissingRule` 只适合开发/迁移期并且必须带 alert；生产安全敏感服务不要启用。

## 执行语义

- 无 server transport：passthrough。
- 缺 rule：默认 fail-closed；只有 `WithFailOpenOnMissingRule` 才 passthrough。
- `AUTHZ_MODE_NONE`：公开跳过授权。
- `AUTHZ_MODE_CHECK`：必须有 subject、action、resource type；nil authorizer 或 check error 返回 503；deny 返回 403。
- subject 由调用方通过 `WithSubjectFunc(...)` 提取；多认证来源时可用 `authn.SubjectFromAny(...)` 组合。
- resource id 优先按 proto rule 的 `resource_id_field` 点路径从请求提取；为空时用 `WithDefaultResourceID`。
- `CheckRequest` 直接携带 `Subject`、`Action`、`ResourceType`、`ResourceID`，具体后端自行转换成 object/principal 形态。

`WithAuditor` 在 allowed/denied/error 三条路径均直接 `Auditor.Emit` CloudEvents，分别对应三个事件类型：

- `servora.authz.allowed.v1`（`emitAuthzAllowed`）
- `servora.authz.denied.v1`（`emitAuthzDenied`）
- `servora.authz.error.v1`（`emitAuthzError`）

三者均使用 `authzauditpb.AuthzDecision` proto，其中 `Decision` 字段为 enum（ALLOWED/DENIED/ERROR），并写入 `authid` extension 供平台投影 actor_id。不再区分 severity WARN/ERROR。这里没有旧版 runtime detail 或 context holder。

## 子包职责

- `openfga/`：OpenFGA backend，含 check/batch/list/tuple 写入与缓存。
- `batch/`、`lister/`：可选能力接口和 helper。
- `noop/`：测试/占位 authorizer。

`authz` 主包只依赖 `Authorizer` 接口；不要反向 import `openfga`。

## 常见反模式

- 在 authz 中解析 Authorization 或 API key。
- 缺 rule 时为了兼容直接放行。
- 把 relation/object 拼接规则散落到业务 middleware。
- 在 `Authorizer.Check` 里做认证、审计后端 fanout 或 transport header 解析。

## 测试

```bash
go test ./security/authz/...
```

关键覆盖：missing rule fail-closed/fail-open、mode none/check、subject/resource 提取、timeout、backend error、allowed/denied/error CloudEvents audit、OpenFGA wrapper。
