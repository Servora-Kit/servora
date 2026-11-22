# Servora Web 软件包

供 [Servora-Kit](https://github.com/Servora-Kit) Web 应用共享的前端基础包。

## 包清单

| 包 | npm | 说明 |
|---|---|---|
| [`@servora/proto-utils`](./packages/proto-utils/) | [![npm](https://img.shields.io/npm/v/@servora/proto-utils)](https://www.npmjs.com/package/@servora/proto-utils) | Proto/Kratos 传输契约工具：传输类型与路径编码、CRUD 辅助函数、FieldMask、ProtoJSON 类型与结构化错误 |

## 安装

```bash
pnpm add @servora/proto-utils
```

## 使用

生成的 HTTP 客户端按需从 `@servora/proto-utils/transport` 导入 `ClientTransport` 等传输接口与路径编码函数。应用在组合根中使用原生 fetch、Next.js fetch、Nuxt `$fetch`、ofetch、Axios 或其它自有 HTTP 客户端实现该适配器：

```typescript
import { createUserHTTPServiceClient } from './generated/example/service/v1'
import { transport } from './api/transport'

export const userApi = createUserHTTPServiceClient(transport)
```

应用适配器负责基础 URL、认证、超时、重试、响应解码和第三方错误识别。JSON 一元请求必须发送 `Accept: application/json`，仅在生成的请求体非空时增加 `Content-Type: application/json`，并原样发送该 ProtoJSON 请求体，不得再次序列化。

通过明确的子路径导入纯辅助函数与框架中立的错误合同：

```typescript
import { ApiError, parseKratosError } from '@servora/proto-utils/errors'
import { firstPage, makeUpdateMask } from '@servora/proto-utils/crud'

try {
  await userApi.GetUser({ name: 'tenants/demo/users/ada' })
} catch (error: unknown) {
  if (error instanceof ApiError) {
    const body = parseKratosError(error)
    // 在这里把 body?.reason 映射为应用 i18n 键或本地文案。
  }
}
```

`@servora/proto-utils/errors` 只描述已经发生的错误。它不发送请求、不刷新令牌、不重放 401 请求、不识别 Axios/ofetch 错误，也不决定 Toast 和用户可见文案。

`ApiError.kind` 区分 `http`、`network`、`timeout` 与 `cancelled`，并保留 `service`、`method` 和原始 `cause`；只有 HTTP 响应错误参与 Kratos reason 解析。

启用 `protoc-gen-go-errors target=ts` 后，错误 sidecar 会从同目录 `index.ts` 复用 Proto enum 联合类型，并生成同名运行时对象与精确 guard。框架 reason 通过语义化子路径公开：

```typescript
import {
  CrudErrorReason,
  isCrudErrorReason,
} from '@servora/proto-utils/error-reasons/servora/crud/v1/errors'

const messages: Readonly<Partial<Record<CrudErrorReason, string>>> = {
  [CrudErrorReason.CRUD_ERROR_REASON_INVALID_FILTER]: '筛选条件无效',
}

export function crudErrorMessage(reason: unknown): string | undefined {
  return isCrudErrorReason(reason) ? messages[reason] : undefined
}
```

业务 Proto 生成的 reason sidecar 应从应用自己的生成目录导入。生成器只提供 enum 运行时事实与 membership guard；本地化、fallback 和 UI 行为仍由应用负责。

## 本地开发

这些包位于 [`servora`](https://github.com/Servora-Kit/servora) 仓库中。本地开发时执行：

```bash
# 在 servora-kit 工作区根目录执行。
pnpm install
```

kit 工作区中的 pnpm 会链接本地 `servora/web/packages/proto-utils` 包。独立使用时从 npm 安装 `@servora/proto-utils`；纯能力通过 `./transport`、`./crud`、`./errors`、`./error-reasons/*` 与 `./proto/*` 子路径公开。本地工作区开启 `linkWorkspacePackages: true` 后会自动建立源码符号链接，其作用类似 Go 的 `go.work` replace 指令。

## 许可证

[MIT](./LICENSE)
