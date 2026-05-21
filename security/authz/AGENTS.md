# AGENTS.md - security/authz/

<!-- Parent: ../AGENTS.md -->
<!-- Updated: 2026-05-21 -->

## 模块定位

`security/authz` 是资源级授权 middleware。它消费 `protoc-gen-servora-authz` 生成的 `AuthzRule`，从 ctx/req 提取 subject 与 resource，调用 `Authorizer.Check`，并在 deny/error 时可选发 CloudEvents audit。

本包不做身份认证、不解析 JWT/API key、不管理 OpenFGA tuple。具体引擎在 `openfga/` 等子包。

## 公开边界

```go
type Authorizer interface {
    Check(ctx context.Context, req CheckRequest) (bool, error)
}

type CheckRequest struct {
    Subject  string
    Relation string
    Object   string
}

type AuthzRule struct {
    Mode           authzv1.AuthzMode
    ObjectType     string
    Relation       string
    ResourceIDPath string
}

func Server(a Authorizer, opts ...Option) middleware.Middleware
func WithRules(rules map[string]*AuthzRule) Option
func WithRulesFuncs(fns ...func() map[string]*AuthzRule) Option
func MergeRules(maps ...map[string]*AuthzRule) map[string]*AuthzRule
func WithDefaultResourceID(id string) Option
func WithCheckTimeout(d time.Duration) Option
func WithFailOpenOnMissingRule(alert func(context.Context, string)) Option
func WithSubjectFunc(fn func(context.Context) (string, bool)) Option
func WithAuditOnDeny(a audit.Auditor) Option
```

`WithFailOpenOnMissingRule` 只适合开发/迁移期并且必须带 alert；生产安全敏感服务不要启用。

## 执行语义

- 无 server transport：passthrough。
- 缺 rule：默认 fail-closed；只有 `WithFailOpenOnMissingRule` 才 passthrough。
- `AUTHZ_MODE_NONE`：公开跳过授权。
- `AUTHZ_MODE_CHECK`：必须有 subject、relation、object；nil authorizer 或 check error 返回 503；deny 返回 403。
- subject 默认从 `core/actor`/ctx 推导；多认证来源时用 `WithSubjectFunc(authn.SubjectFromAny(...))`。
- resource id 优先 `ResourceIDPath` 点路径提取；为空时用 `WithDefaultResourceID`。
- object 形态由 `ObjectType + ":" + resourceID` 组成，principal 形态由 subject func 决定。

`WithAuditOnDeny` 在 deny/error 路径直接 `Auditor.Emit` CloudEvents：deny severity `WARN`，check error severity `ERROR`。这里没有旧版 runtime detail 或 context holder。

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

关键覆盖：missing rule fail-closed/fail-open、mode none/check、subject/resource 提取、timeout、backend error、deny/error CloudEvents audit、OpenFGA wrapper。
