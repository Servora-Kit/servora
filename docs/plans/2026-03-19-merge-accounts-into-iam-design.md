# 设计文档：将 web/accounts 合并到 web/iam（完整重构）

**日期：** 2026-03-19
**状态：** 已审批

## 动机

`web/accounts` 是一个轻量 SPA（Vite + React + TanStack Router），负责 OIDC/OAuth 认证页面：带 `authRequestID` 的登录、注册、邮箱验证和密码重置。`web/iam` 是功能完备的管理端前端（TanStack Start + Router + Query + Table + Form + Store），已有自己的 `/_auth/` 布局，包含登录、注册、注册成功、邮箱验证和回调路由。

两个应用在认证页面上高度重叠，但存在明显差距。合并后消除重复代码库、统一样式，并简化开发与部署结构。借此机会对所有 `_auth/` 路由和后端 OIDC 代码做全面重构。

## 方案

**单一 `/login` 路由按参数分支** — 主流 IdP 的标准做法。登录表单完全相同，仅认证成功后的行为根据 `authRequestID` 是否存在而分叉。

## 变更说明

### 1. 前端基础设施

#### 1a. Vite 代理 — 统一走 Traefik 网关（:8080）

在现有 `/v1` 代理基础上，补充 OIDC 相关路径（`/oauth`、`/login`、`/authorize`、`/.well-known`），统一指向 `http://127.0.0.1:8080`。

`/login` 后端路由与前端路由 `/_auth/login` 无冲突。

#### 1b. 新增共享 hook：`useResendVerification`

`login.tsx`、`register-success.tsx`、`verify-email.tsx` 三处均有几乎相同的「重发验证邮件」逻辑。抽取为 `#/hooks/use-resend-verification.ts`：

```typescript
function useResendVerification(email: string): {
  resend: () => Promise<void>
  resending: boolean
  message: string
}
```

三处统一引用，消除重复。

#### 1c. 新增共享 Kratos 错误解析工具（`@servora/web-pkg/errors`）

所有服务共用 Kratos 错误格式 `{ code, reason, message, metadata? }`。在 `web/pkg/errors.ts` 新增：

```typescript
interface KratosErrorBody { code: number; reason: string; message: string; metadata?: Record<string, string> }
function parseKratosError(err: ApiError): KratosErrorBody | null   // 提取结构化错误体
function isKratosReason(err: ApiError, reason: string): boolean    // 判断特定 reason
function kratosMessage(err: ApiError, fallback?: string): string   // 提取用户可读消息，含 network/timeout 降级
```

各前端通过 `import { parseKratosError, isKratosReason, kratosMessage } from '@servora/web-pkg/errors'` 使用。

同步简化 `web/iam/src/lib/toast.ts`：移除内联 `ApiErrorBody` / `extractMessage`，改用 `kratosMessage`。

### 2. 前端路由重构

#### 2a. `/_auth/login` — OIDC 支持 + useReducer 重构

**搜索参数：** 新增可选 `authRequestID`。

**状态重构：** 6 个 `useState` → `useReducer`：
- State: `{ email, password, status, error, emailNotVerified, resendMsg }`
- Actions: `SET_FIELD | SUBMIT | SUBMIT_ERROR | EMAIL_NOT_VERIFIED | RESEND_START | RESEND_DONE | RESET`

**登录成功后分支：**
- 有 `authRequestID` → 调 `POST /login/complete` → `window.location.href = callbackURL`（不存 JWT，不校验 admin 角色）
- 无 `authRequestID` → 现有 JWT 登录 + admin 角色验证

**UI：** 在「没有账号？」旁新增「忘记密码？」链接，指向 `/_auth/reset-password`。

使用 `useResendVerification` hook 替换内联重发逻辑。使用 `kratosMessage` / `isKratosReason` 替换 cast。

#### 2b. `/_auth/register` — useReducer 重构 + Cap widget 修复

**状态重构：** 6 个 `useState` → `useReducer`，新增 `capResetKey` 状态。

**Cap widget 重置：** 移除 DOM 操作（`cloneNode + replaceChild`），改用 React key 强制重新挂载：
```tsx
<cap-widget key={state.capResetKey} ... />
```
验证码过期时 `dispatch({ type: 'RESET_CAP' })` 递增 key。

使用 `kratosMessage` / `isKratosReason` 替换 cast。

#### 2c. `/_auth/verify-email` — lucide 图标 + 共享 hook

**SVG 替换：** 内联 SVG → `lucide-react` 图标（`Loader2`、`CheckCircle2`、`XCircle`）。

使用 `useResendVerification` hook 替换内联重发逻辑。使用 `kratosMessage` / `isKratosReason` 替换 cast。

#### 2d. `/_auth/register-success` — lucide 图标 + 共享 hook

**SVG 替换：** 内联 Mail SVG → `lucide-react` 的 `Mail` 图标。

使用 `useResendVerification` hook 替换内联重发逻辑。

#### 2e. 新增路由：`/_auth/reset-password`

用 IAM 的 shadcn/ui 组件 + `_auth` 布局。根据 `token` 搜索参数切换两种视图：
- 无 token → 邮箱输入，调用 `iamClients.authn.RequestPasswordReset`
- 有 token → 新密码 + 确认密码，调用 `iamClients.authn.ResetPassword`

使用 `useReducer` + `kratosMessage` / `isKratosReason`。

### 3. 后端重构

#### 3a. Proto：`conf.v1.App.Oidc` 新增 `login_base_url`

```protobuf
string login_base_url = 4; // OIDC 登录页基地址；GET /login 会 302 到 {login_base_url}/login?authRequestID=...
```

#### 3b. `oidc/login.go` 全面重构

**配置注入：** `NewLoginHandler` 接收 `*conf.App`，从配置读取 `login_base_url`，缺失时 startup panic。

**Proto 生成错误：** `authenticate` 不再返回 `fmt.Errorf` 字符串，直接使用 `authnpb` 生成的错误函数：
- `authnpb.ErrorInvalidCredentials("...")` — 邮箱/密码错误（401）
- `authnpb.ErrorTokenExpired("...")` — authRequest 过期（401）
- Kratos `errors.BadRequest(...)` — 缺少必填字段（400）
- 内部异常（Redis/DB）返回 Kratos `errors.InternalServer(...)`，不暴露给客户端

handler 层用 `errors.FromError(err)` 提取 `Code` / `Reason` / `Message`，序列化为 `{"code": 401, "reason": "INVALID_CREDENTIALS", "message": "..."}` — 与所有 IAM proto API 错误响应格式完全一致，无需手动构建 JSON。

**错误处理：** `json.Marshal(reqData)` 不再忽略错误。

#### 3c. 配置文件

`configs/local/bootstrap.yaml` 和 `configs/docker/bootstrap.yaml` 的 `app.oidc` 节点均新增：
```yaml
login_base_url: "http://localhost:3000"
```

### 4. 删除与清理

- 删除 `web/accounts/` 整个目录
- `Makefile`：`web.dev` 目标移除 `--filter "./web/accounts"`
- 运行 `pnpm install` 刷新 lockfile
- 运行 `make gen`（proto 有变更）+ `make wire`（`NewLoginHandler` 签名变更）
- 更新 AGENTS.md / CLAUDE.md，删除 accounts 相关描述
