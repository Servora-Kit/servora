# AGENTS.md - servora/web

<!-- Updated: 2026-06-10 -->

## 项目概览

`servora/web` 是 `servora` 主仓内的前端共享包工作区，提供跨业务仓库复用的前端基础设施。当前以一个 npm 包承载多个 subpath export，减少发布和下游依赖管理成本。

当前包含：

| 包 | 路径 | 说明 |
|---|------|------|
| `@servora/proto-utils` | `web/packages/proto-utils/` | Proto 查询工具、FieldMask、通用 HTTP client、Token 管理、Kratos 错误解析 |

## 开发约束

### 提交规范

格式：`type(scope): description`。type：`feat`/`fix`/`refactor`/`docs`/`test`/`chore`。scope 建议：`proto-utils`、`client`、`repo`。

### 包设计原则

- 只放**跨业务仓库复用**的前端基础能力
- 保持 API 小而稳定：导出的类型/函数是多个应用的共享契约
- 不要放页面状态、业务 store、路由逻辑、特定服务的 toast/文案

### 与业务仓库的关系

- 业务仓库通过 `@servora/proto-utils` 依赖本包；client 能力通过 `@servora/proto-utils/client` 子路径暴露
- 本地开发：顶层 `pnpm-workspace.yaml`（在 `servora-kit/`）联调，`workspace:*` 自动 link 本地源码
- CI/生产：通过 npm 公共 registry 安装（`npm publish --provenance`）

### 发布流程

```bash
cd web/packages/proto-utils
# 1. 修改代码 → 提交
# 2. 更新 package.json 中的 version
# 3. 打 tag（格式 proto-utils/v<version>）
git tag proto-utils/v0.1.0
git push origin main --tags
# GitHub Actions 自动构建并 npm publish --provenance
```

业务仓库更新：将 `"@servora/proto-utils": "workspace:*"` 改为 `"@servora/proto-utils": "^0.1.0"`（CI 独立构建时需要），本地开发仍通过 workspace link 使用本地源码。

## 目录结构

```
servora/
├── .github/
│   └── workflows/
│       └── publish-proto-utils.yml # tag proto-utils/v* 触发自动发布到 npm
├── api/protos/               # Servora 内建 proto 来源
├── buf.typescript.gen.yaml   # 生成内建 proto 的 TS 类型
└── web/
    ├── packages/
    │   └── proto-utils/      # @servora/proto-utils
    │       ├── src/
    │       │   ├── query.ts
    │       │   └── client/
    │       ├── dist/         # 构建产物（gitignored，npm publish 时生成）
    │       ├── package.json
    │       ├── tsconfig.json
    │       └── tsconfig.build.json
    ├── pnpm-workspace.yaml
    └── package.json
```

## packages/proto-utils 修改约定

- 优先把这里当作 **proto 工具与 client 适配层**，而不是业务逻辑目录
- 新增能力前先判断它是否能被多个前端应用复用
- `packages/proto-utils/src/gen/` 由 `servora/buf.typescript.gen.yaml` 生成，不要手改
- 错误处理需兼容 Kratos 返回格式：`{ code, reason, message, metadata? }`
- 修改 Token 刷新、请求头注入、`onError` 触发时机时，必须检查是否影响现有调用方
- React、Vue、TanStack Query 等框架适配通过 subpath export 隔离，相关依赖必须使用 optional peer dependencies

## 禁止事项

- 不要放某个服务专属的 toast / 文案 / 页面跳转策略
- 不要在这里复制业务仓库 `api/gen/ts/` 里的生成类型
- 不要把页面组件、UI 库放到这里（UI 共享另行处理）
