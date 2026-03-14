# IAM 前端

IAM 服务的管理后台前端，位于仓库根目录 `web/iam/`。

## 技术栈

**简写**：Vite + TanStack + Tailwind v4 + shadcn/ui

**完整**：

| 层级           | 选型                    | 说明 |
|----------------|-------------------------|------|
| 构建/运行时    | React + Vite            | SPA，开发与构建 |
| 路由           | TanStack Router         | 类型安全、file-based 路由 |
| 服务端状态     | TanStack Query          | 接口请求、缓存、加载/错误态 |
| 表单           | TanStack Form           | 表单状态与校验 |
| 样式           | Tailwind CSS v4         | 工具类 + CSS 变量 |
| 组件/设计系统  | shadcn/ui               | 无头组件 + Radix 原语，可定制 |
| 图标           | lucide-react            | 通用图标 |
| 客户端状态     | zustand                 | 登录态、UI 状态、筛选等，与 TanStack Query 分工 |

## 主题与配色

- **明暗切换**：shadcn/ui 通过 CSS 变量 + `dark` class 支持日/夜模式，使用官方 ThemeProvider 即可。
- **Catppuccin**：使用 [catppuccin/shadcn-ui](https://github.com/catppuccin/shadcn-ui) 提供的主题变量，将 `themes/` 下对应 flavor（如 Latte 亮 / Mocha 暗）的 CSS 复制到 `globals.css` 的 `:root` 与 `.dark` 中。

## 目录约定

- 前端应用根目录：`web/iam/`（与 `app/iam/service/` 后端并列，不放在 service 下）。
- 后续若其他服务需要独立前端，可沿用 `web/<服务名>/` 的布局。

## 参考

- Kemate：同栈（TanStack Router + Query + zustand + Tailwind + Radix/shadcn 风格），客户端状态与请求封装可参考其 `useAuthStore`、`queryClient`、API 封装方式。
- TanStack Router 与 shadcn/ui 集成：[How to Integrate TanStack Router with Shadcn/ui](https://tanstack.com/router/latest/docs/how-to/integrate-shadcn-ui)。
