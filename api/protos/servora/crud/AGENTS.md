# AGENTS.md - api/protos/servora/crud/

<!-- Parent: ../../AGENTS.md -->
<!-- Updated: 2026-07-21 -->

## 当前定位

`api/protos/servora/crud/` 定义 Servora CRUD 框架公共协议，由 `core/crud` runtime 消费并随现有 Buf module 发布。

这里不存放业务资源的 request/response；业务资源 Proto 应在自身 package 中直接声明标准 List 与 CRUD 方法字段。

## 当前结构

```text
crud/
├── AGENTS.md
└── v1/
    ├── errors.proto      # CRUD 框架错误枚举
    └── page_token.proto  # 服务端 page token 私有载荷
```

## 契约边界

- `CrudErrorReason` 只包含 CRUD 框架自身产生的输入错误与内部错误；存储事实和业务语义使用业务错误 Proto。
- `PageTokenPayload` 与 `CursorValue` 是服务端实现载荷；业务 request/response 不得 import 或嵌套它们。
- 客户端合同只允许原样回传 opaque `page_token`，不得依赖载荷结构。
- package、目录和 `go_package` 必须保持 `servora.crud.v1` / `servora/crud/v1` 对齐。

## 生成与校验

在 `servora/` 执行：

```bash
make lint.proto
make gen
make gen.ts
```

生成代码只由 Buf/Makefile 写入，不手改 `api/gen` 或 TypeScript 生成目录。
