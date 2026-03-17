# TODO

## OIDC 登录页：可配置登录页基地址（前端接管 UI）

- **背景**：当前 IAM 微服务内嵌 GET/POST `/login` 的 SSR 登录页；`web/iam` 前端将接管登录 UI，微服务只保留 POST `/login/complete` API。当前端与 IAM 不同域时（如前端 `https://admin.example.com`、IAM `https://iam.example.com`），需要可配置的「登录页基地址」。
- **目标**：支持通过配置指定登录页的完整基地址，使 OIDC 授权时重定向到前端域名下的登录页，而不是 IAM 域下的 `/login`。
- **待办**：
  1. **配置扩展**：在 `conf.App.Oidc` 中新增 `login_url_base`（可选 string）。例如 `https://admin.example.com`，不设则沿用当前行为（相对路径 `/login`，即 IAM 同域）。
  2. **LoginURL 行为**：在 `op.Client` 适配器（`internal/data/oidc_client.go`）的 `LoginURL(requestID)` 中，若配置了 `login_url_base`，则返回 `{login_url_base}/login?authRequestID={requestID}`（注意拼接时避免双斜杠）；否则返回 `/login?authRequestID={requestID}`。需将 `login_url_base` 注入到 oidcStorage/oidcClient（例如从 `*conf.App` 或 data 层构造时传入）。
  3. **callbackURL 为绝对 URL**：`internal/oidc/login.go` 中 `authenticate` 返回的 callbackURL 当前为相对路径 `/authorize/callback?id=xxx`。当登录页在前端且与 IAM 不同域时，前端拿到后需要跳转到 IAM 域；建议后端根据 `app.external_url` 返回绝对 URL，例如 `{external_url}/authorize/callback?id={authRequestID}`，这样前端无需关心 IAM 域名，直接 `location.href = callbackURL` 即可。
  4. **微服务侧去掉 HTML**：前端实现登录页并稳定使用 POST `/login/complete` 后，移除 IAM 内 GET `/login` 与 POST `/login` 的 SSR/表单处理逻辑，仅保留 POST `/login/complete`；可选保留 GET `/login` 为 302 重定向到 `login_url_base/login?authRequestID=xxx`，便于旧链接或同域时统一跳转到前端。
  5. **文档**：在 `web/iam/README.md` 或 IAM 开发文档中说明登录流程（授权 → 重定向到前端登录页 → 前端调 POST `/login/complete` → 跳转 callbackURL 完成 OAuth），以及 `login_url_base` 与 `external_url` 的配置含义。

---

## IAM OIDC e2e 测试

- **放置位置**：按服务设 e2e，不放在仓库根。例如 IAM 的 e2e 放在 `app/iam/service/e2e/`（与 `cmd/`、`internal/` 平级），包名 `e2e`。若后续有跨多服务的整条链路验证，再在根目录增加 `e2e/`。
- **覆盖范围**：Authorization Code 全流程（创建 Application → /authorize → 登录 → 拿 code 换 token → /userinfo、/oauth/introspect）；Client Credentials 换 token；Discovery `/.well-known/openid-configuration`。依赖可用 testcontainers（Postgres + Redis）或复用 `make compose.dev` 后指向依赖地址。
- **何时正式进行**：在 **P1 OIDC 接口与授权流程基本稳定、不再做大改** 时落地（例如登录页基地址、前端接管登录等方案定稿并实现后）。可约定为「准备将 IAM 纳入正式交付或 CI 回归前」必须通过 e2e；日常开发仍以单元测试与本地手动验证为主。Makefile 可提供 `make test.e2e`（或需显式环境变量开启），默认不参与 `make test`，避免 CI 强依赖 compose/容器。

---

## IAM 前端 authStore 会话持久化优化

- **背景**：当前 `authStore` 仅持久化 token，不持久化 `currentUser`。刷新页面后 Sidebar 的用户卡片和角色判断（超管/租户管理员）需要等 API 返回才能渲染，导致短暂空白闪烁。参考 Kemate 的做法（`useAuthStore` 将 `currentUser` 和 `capabilities` 也序列化到 localStorage）。
- **待办**：
  1. **`currentUser` 持久化**：在 `setUser` 时将用户信息 JSON 序列化写入 localStorage，`authStore` 初始化时从 localStorage 反序列化恢复。刷新页面后 Sidebar 可立即渲染用户名、邮箱、角色，无需等待 API。
  2. **防重设值**：`setTokens` 增加与当前值对比，相同时跳过写入，减少不必要的 store 更新和 localStorage 写入。
  3. **Capabilities 缓存（后续）**：等实际接入 OpenFGA 权限驱动导航后，再将 capabilities map 持久化到 localStorage，使刷新页面后导航菜单可立即渲染。
