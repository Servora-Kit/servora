# IAM Phase 10 实施计划

<!-- Created: 2026-03-19 -->

## 概述

本文档是 IAM 统一身份平台的 Phase 10 实施计划，是 [Phase 1–9 重构](./2026-03-18-iam-identity-platform-plan.md) 完成后的后续独立增量。

Phase 10 聚焦于三个方向：
1. **M2M 能力** — Client Credentials Grant + OpenFGA 服务间授权
2. **公共账户前端** — `web/accounts/` 独立登录/注册应用
3. **通用中间件上提** — `pkg/authn`、`pkg/authz` 供其他微服务复用

每个 Task 均可独立执行，没有严格依赖顺序。

---

## Task A: M2M Client Credentials Grant

**目标：** 让机器服务（另一个微服务）能用 `client_id + client_secret` 直接获取 Access Token，无需用户介入。

**当前状态：** `oidc_storage.go` 已实现 `ClientCredentials` 和 `ClientCredentialsTokenRequest`，但尚未经过测试。

### 步骤

**Step 1：** 手动验证 Client Credentials 流程

```bash
# 创建 m2m 类型应用
ACCESS_TOKEN=<admin-token>
APP=$(curl -s -X POST http://localhost:8000/v1/applications \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "m2m-service",
    "type": "m2m",
    "grant_types": ["client_credentials"],
    "scopes": ["openid"]
  }')
CLIENT_ID=$(echo "$APP" | jq -r '.application.clientId')
CLIENT_SECRET=$(echo "$APP" | jq -r '.clientSecret')

# 获取 token
curl -s -X POST http://localhost:8000/oauth/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=client_credentials&client_id=$CLIENT_ID&client_secret=$CLIENT_SECRET&scope=openid"
```

期望返回包含 `access_token` 的 JSON。

**Step 2：** 检查当前 `oidcClient` 对 `client_credentials` grant type 的支持

文件：`app/iam/service/internal/data/oidc_client.go`

确认 `GrantTypes()` 在 `app.GrantTypes` 包含 `client_credentials` 时返回正确。

**Step 3：** 在 E2E 测试脚本中补充 M2M 测试用例

```bash
# 建议写入 manifests/scripts/e2e/iam_m2m_e2e.sh
```

**Step 4：** Commit

```
feat(app/iam): 验证并补充 M2M client_credentials grant E2E 测试
```

---

## Task B: OpenFGA 服务间授权

**目标：** 在 OpenFGA model 中定义 `service` 类型，允许服务在调用其他服务时携带身份，并由被调用服务通过 OpenFGA 校验是否有权限。

**当前状态：** IAM 已提供 OpenFGA 基础设施，但 model 中没有 `service` 类型。

### OpenFGA Model 变更

**Step 1：** 在 `manifests/openfga/model/servora.fga` 中添加 `service` 类型

```fga
# service 代表 M2M 服务身份（service account）
type service
  relations
    define platform: [platform]
    define can_call: [service] or can_call from platform
```

**Step 2：** 更新 `manifests/openfga/tests/servora.fga.yaml`，添加 service 测试用例

```yaml
tuples:
  - user: service:order-service
    relation: can_call
    object: service:payment-service

tests:
  - name: order-service can call payment-service
    check:
      - user: service:order-service
        object: service:payment-service
        assertions:
          can_call: true
  - name: unrelated service cannot call payment-service
    check:
      - user: service:unknown
        object: service:payment-service
        assertions:
          can_call: false
```

**Step 3：** 验证并应用

```bash
make openfga.model.validate
make openfga.model.test
make openfga.model.apply
```

**Step 4：** Commit

```
feat(infra/openfga): 添加 service 类型，支持 M2M 服务间授权
```

---

## Task C: web/accounts/ — 公共账户前端

**目标：** 将 OIDC 登录页（当前是 Go 模板 SSR）替换为独立 React 应用，放在 `web/accounts/`。同时提供用户自助注册、重置密码等页面。

**技术栈：** React + TanStack Router + shadcn/ui + Catppuccin（与 `web/iam/` 共享风格）

### 目录结构

```
web/accounts/
├── package.json           # @servora/accounts
├── tsconfig.json
├── vite.config.ts
├── src/
│   ├── main.tsx
│   ├── router.tsx
│   ├── api.ts             # 调用 IAM API
│   ├── routes/
│   │   ├── login.tsx      # OIDC 登录页（替换 Go 模板）
│   │   ├── register.tsx   # 用户注册
│   │   ├── verify-email.tsx
│   │   └── reset-password.tsx
│   └── components/
│       └── auth-card.tsx
```

### 步骤

**Step 1：** 创建 `web/accounts/` 应用脚手架

```bash
cd web/accounts
pnpm create vite . --template react-ts
```

**Step 2：** 配置 `pnpm-workspace.yaml` 纳管（已纳管 `web/*`，无需修改）

**Step 3：** 实现登录页

- GET `/login?authRequestID=xxx` → 渲染登录表单
- POST `/login`（发送 form-encoded `authRequestID + email + password`）
- 替换 `app/iam/service/internal/oidc/login.go` 中的 Go 模板为重定向到 `web/accounts/` 托管的静态页

**Step 4：** 实现注册/重置密码页

调用现有 IAM API：
- `POST /v1/auth/signup/email` — 注册
- `GET /v1/auth/email/verify?token=xxx` — 邮箱验证
- `POST /v1/auth/password/reset/request` — 请求重置
- `POST /v1/auth/password/reset` — 执行重置

**Step 5：** 配置 Vite 开发代理到 IAM 服务

**Step 6：** Commit

```
feat(web/accounts): 新建公共账户前端 — 登录/注册/重置密码页
```

---

## ~~Task D: web/ui/ — 共享 UI 组件包~~ → 已调整，见 Task D'

**目标：** 将 `web/iam/` 中的 shadcn/ui 组件、Catppuccin 主题配置提取为 pnpm workspace 包，供 `web/accounts/` 复用。

### 目录结构

```
web/ui/
├── package.json           # @servora/ui
├── tsconfig.json
├── src/
│   ├── index.ts           # 导出所有组件
│   ├── components/        # shadcn/ui 基础组件
│   ├── theme/             # Catppuccin 主题 CSS 变量
│   └── utils/
│       └── cn.ts          # tailwind-merge + clsx
```

### 步骤

**Step 1：** 创建 `web/ui/` 包

**Step 2：** 将 `web/iam/src/components/ui/` 中的基础 shadcn 组件迁移到 `web/ui/`

**Step 3：** 将 `web/iam/src/index.css` 中的 Catppuccin CSS 变量提取到 `web/ui/theme/`

**Step 4：** `web/iam/` 和 `web/accounts/` 均引用 `@servora/ui`

**Step 5：** Commit

```
feat(web/ui): 提取共享 UI 组件包（shadcn + Catppuccin）
```

---

## Task E: pkg/authn — 通用 JWT 验签中间件

**目标：** 将 `app/iam/service/internal/server/middleware/authn.go` 中的 Authn 中间件上提到 `pkg/authn`，供其他微服务（如 sayhello）直接使用，无需重写。

**当前状态：** Authn 中间件已在 IAM 内部实现，依赖 `pkg/jwks.KeyManager`。

### 设计

```go
// pkg/authn/middleware.go
package authn

import (
    "github.com/go-kratos/kratos/v2/middleware"
    "github.com/Servora-Kit/servora/pkg/jwks"
)

type Option func(*options)

func WithVerifier(v *jwks.Verifier) Option { ... }
func WithSkipFunc(f func(op string) bool) Option { ... }

// Authn 返回 Kratos 中间件，从 Authorization: Bearer <token> 提取并验签 JWT，
// 将 actor 信息注入 context。
func Authn(opts ...Option) middleware.Middleware { ... }
```

调用方只需：
```go
import "github.com/Servora-Kit/servora/pkg/authn"

mw = append(mw, authn.Authn(authn.WithVerifier(km.Verifier())))
```

### 步骤

**Step 1：** 在 `pkg/authn/` 创建包，将逻辑从 IAM internal 提取

**Step 2：** 为 `pkg/authn` 编写单元测试

**Step 3：** 更新 `app/iam/service/internal/server/middleware/authn.go` 改为调用 `pkg/authn`

**Step 4：** 在 `app/sayhello/service/` 中接入 `pkg/authn`

**Step 5：** Commit

```
feat(pkg/authn): 提取通用 JWT 验签中间件，IAM 和 sayhello 均可复用
```

---

## Task F: pkg/authz — 通用 OpenFGA check 中间件

**目标：** 将 `app/iam/service/internal/server/middleware/authz.go` 的核心逻辑上提到 `pkg/authz`，使其他微服务可以按相同的 proto 注解驱动方式执行 OpenFGA check。

**注意：** 其他业务服务（如订单、支付）有自己的 OpenFGA model，需要各自定义 `AuthzRules`，不依赖 IAM 的规则。

### 设计

```go
// pkg/authz/middleware.go
package authz

import (
    "github.com/go-kratos/kratos/v2/middleware"
    "github.com/Servora-Kit/servora/pkg/openfga"
)

type AuthzRule struct {
    Mode       string // "none" | "check"
    ObjectType string
    Relation   string
}

type Option func(*options)

func WithFGAClient(c *openfga.Client) Option { ... }
func WithAuthzRules(rules map[string]*AuthzRule) Option { ... }
func WithAuthzCache(rdb *redis.Client, ttl time.Duration) Option { ... }

// Authz 返回 Kratos 中间件，根据 AuthzRules 对每个 operation 执行 OpenFGA check。
func Authz(opts ...Option) middleware.Middleware { ... }
```

### 步骤

**Step 1：** 创建 `pkg/authz/` 包，提取 IAM authz 中间件核心逻辑

**Step 2：** 为 `pkg/authz` 编写单元测试（复用 IAM 中现有 authz_test.go 的测试用例）

**Step 3：** 更新 `app/iam/service/internal/server/middleware/authz.go` 改为调用 `pkg/authz`

**Step 4：** 文档化用法，说明其他服务如何定义自己的 `AuthzRules`（proto gen）并接入

**Step 5：** Commit

```
feat(pkg/authz): 提取通用 OpenFGA check 中间件，支持 proto 注解驱动鉴权
```

---

## Task D': @servora/api-client 共享 request handler ✅ 已完成（2026-03-19）

**背景：** `requestHandler.ts`（含 `createRequestHandler`、`ApiError`、`TokenStore` 等）是与 proto 生成代码配套的通用逻辑，与具体业务无关，应放在 `@servora/api-client` 包内，供所有前端复用（避免每个 app 各自复制一份）。

**已完成的变更：**

- `api/ts-client/request.ts` — 从 `web/iam/src/service/request/requestHandler.ts` 提取，作为 `@servora/api-client/request` 导出
- `api/ts-client/package.json` — 声明 `ofetch` 为 dependency（pnpm 已安装）
- `web/iam/src/service/request/clients.ts` — 改为从 `@servora/api-client/request` 导入
- `web/iam/src/lib/toast.ts` — 同上
- `web/iam/src/service/request/requestHandler.ts` — 已删除

**消费方用法（适用于所有前端 app）：**

```typescript
import { createRequestHandler } from '@servora/api-client/request'
import type { TokenStore, RequestHandlerOptions, ApiError } from '@servora/api-client/request'
import { createAuthnServiceClient, createUserServiceClient } from '@servora/api-client/iam/service/v1'

const handler = createRequestHandler({
  baseUrl: import.meta.env.VITE_API_BASE_URL,
  tokenStore,       // 实现 TokenStore 接口
  autoRefreshToken: true,
  onError(err) { /* 全局错误处理 */ },
})

const authn = createAuthnServiceClient(handler)
const user = createUserServiceClient(handler)
```

**注意：** 各前端 `app` 的 `clients.ts`（组装具体 service）仍需自己维护，因不同 app 需要不同的 service 组合。

---

## 优先级建议

| Task | 优先级 | 依赖 | 预估工作量 | 状态 |
|------|--------|------|------------|------|
| D': @servora/api-client request handler | — | — | 已完成 | ✅ |
| A: M2M Client Credentials | 高 | 无 | 0.5 天（已有实现，只需验证和测试） | 待实施 |
| B: OpenFGA service 授权 | 中 | 无 | 0.5 天 | 待实施 |
| E: pkg/authn | 高 | 无 | 1 天 | 待实施 |
| F: pkg/authz | 高 | E | 1.5 天 | 待实施 |
| C: web/accounts/ | 中 | D' | 3 天 | 待实施 |
| D: web/ui/ | 低 | C | 1 天 | 待实施 |

**推荐实施顺序：** A → E → F → B → C → D

Task A 最快出成果，Task E/F 对其他微服务接入帮助最大。

---

## 当前基线状态（Phase 9 完成后）

- ✅ IAM 服务编译并运行正常
- ✅ 用户注册/登录/刷新 token
- ✅ 邮箱验证/密码重置
- ✅ 应用 CRUD（无 tenant 依赖）
- ✅ OIDC 完整流程（authorize → login → code → token → userinfo → refresh）
- ✅ JWKS 端点 + OIDC Discovery
- ✅ platform admin 鉴权（ListUsers/GetUser/ListApplications/CreateApplication）
- ✅ IAM admin console（非管理员无法登录）
- ✅ OpenFGA model（platform + application，测试 7/7 通过）
