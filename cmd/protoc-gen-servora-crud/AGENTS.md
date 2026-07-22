# AGENTS.md - cmd/protoc-gen-servora-crud/

<!-- Parent: ../AGENTS.md -->
<!-- Updated: 2026-07-21 -->

## 模块目的

`protoc-gen-servora-crud` 从完整 `google.api.resource` descriptor 生成按源 Proto 文件组织的 Go/TypeScript CRUD companion。

## 核心边界

- Go 输出仅包含 `XxxCRUDDescriptor`、typed `XxxFieldPath`/`XxxFields` 与资源名 Parse/Validate/Format helper；不得 import `core/crud`。
- TypeScript 输出仅包含资源名、字段路径与 update-field 轻量 helper；不得重复 Proto 类型、HTTP client 或 transport 编码。
- 插件按 resource 注解 opt-in；标准 Get/List/Create/Update/Delete 候选在生成期严格校验，错误必须包含方法或字段路径。
- 不生成 service、biz、data、repository、command、ORM schema/setter、授权或事务代码。
- multi-pattern name helper 必须按 skeleton 唯一匹配；Create/List 不得暗选声明顺序中的第一个 pattern。

## 验证

在 `servora/` 执行：

```bash
go test ./cmd/protoc-gen-servora-crud
make gen
make gen.ts
```
