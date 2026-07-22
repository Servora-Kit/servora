# Servora Web Packages

Shared frontend libraries for [Servora-Kit](https://github.com/Servora-Kit) web applications.

## Packages

| Package | npm | Description |
|---------|-----|-------------|
| [`@servora/proto-utils`](./packages/proto-utils/) | [![npm](https://img.shields.io/npm/v/@servora/proto-utils)](https://www.npmjs.com/package/@servora/proto-utils) | Proto/Kratos API utilities: query builders, FieldMask, HTTP client, Kratos errors |

## Installation

```bash
pnpm add @servora/proto-utils
```

## Usage

```typescript
import { createRequestHandler } from '@servora/proto-utils/client/request'
import { parseKratosError, kratosMessage } from '@servora/proto-utils/client/errors'
import { firstPage, makeUpdateMask } from '@servora/proto-utils/crud'
```

## Local Development

These packages live inside the [`servora`](https://github.com/Servora-Kit/servora) repository. For local development:

```bash
# In the servora-kit workspace root.
pnpm install
```

In the kit workspace, pnpm links the local `servora/web/packages/proto-utils` package. On npm, install `@servora/proto-utils`; the `client`, `query`, and future CRUD/React/Vue helpers are exposed as subpath exports. In the local workspace, `linkWorkspacePackages: true` automatically symlinks to the source, equivalent to Go's `go.work` replace directive.

## License

[MIT](./LICENSE)
