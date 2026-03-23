## Why

当前审计规则在各微服务的 `grpc.go` / `http.go` 中以手写 `map[string]audit.Rule{...}` 形式硬编码（如 sayhello 中的 `audit.WithRules(...)`），每新增一个 RPC 都需手动维护。同时，现有 `protoc-gen-servora-authz` 生成的 `AuthzRules` 是可变的 `var`，存在并发安全隐患。

此外，`pkg/` 框架包中存在多处业务特化硬编码，违反约束 #8（pkg 框架包去特化原则）：`pkg/actor` 中硬编码了 tenant/organization/project scope key 常量与便捷方法；`pkg/transport/server/middleware/scope.go` 硬编码了 3 个业务 header；`pkg/authz` 中硬编码了 `"user:"` principal 前缀且只允许 user actor 做 Check；`pkg/actor/user.go` 中存在未使用的 legacy `Metadata` 字段。

Phase 3（master design doc）的目标是让审计规则像授权规则一样走 **all-in-proto 注解 → 代码生成 → middleware 自动执行** 的声明式路线，并顺带改进 authz 插件的输出形式，同时一并清理 pkg 中的所有业务特化代码。本阶段以 sayhello 服务为验证目标。

## What Changes

### Codegen & Proto
- **新建 `cmd/protoc-gen-servora-audit`**：读取 `servora.audit.v1.audit_rule` method option → 生成 `audit_rules.gen.go`（per-service package），输出 `func AuditRules() map[string]audit.Rule`（内部 `var` + 返回 copy，不可变）
- **更新 `annotations.proto`**：`string operation` → `ResourceMutationType mutation_type`（复用已有枚举），注释同步更新
- **更新 `protoc-gen-servora-authz`**：**BREAKING** — `var AuthzRules` → `func AuthzRules() map[string]authz.AuthzRule`（返回 copy），调用方需改 `orderpb.AuthzRules` → `orderpb.AuthzRules()`
- **新建 `buf.audit.gen.yaml`**：审计代码生成模板，集成到 `make api`
- **为 sayhello proto 添加 `audit_rule` 注解**：验证 codegen → 生成 `audit_rules.gen.go` → middleware 自动加载，替换 `grpc.go` 中的手写 `WithRules`
- **生成产物包含 `target_id_field` 解析逻辑**：codegen 为每条规则生成 field path 提取 helper

### Middleware 适配
- **更新 `pkg/audit/middleware.go`**：适配 `mutation_type` 字段、`TargetIDFunc`，支持 `WithRulesFunc`
- **更新 `pkg/authz/authz.go`**：`WithAuthzRulesFunc` + principal 构造泛化 + 多 actor type 支持

### pkg 去特化清理
- **`pkg/actor/user.go`**：删除 `ScopeKeyTenantID` / `OrganizationID` / `ProjectID` 常量 + 6 个便捷方法 + legacy `Metadata` / `Meta()` 字段
- **`pkg/actor/context.go`**：删除 3 个 `XxxIDFromContext` helper → 新增通用 `ScopeFromContext(ctx, key)`
- **`pkg/actor/system.go`**：`ID()` 中 `"system:"` 前缀改为构造方提供完整 ID
- **`pkg/transport/server/middleware/scope.go`**：可配置化 — 接收 `[]ScopeBinding` 映射表替代硬编码 3 个 header
- **`pkg/authz/authz.go`**：`"user:" + userID` 硬编码改为根据 `actor.Type()` 动态构造 principal；移除 `TypeUser` 硬判断，支持多种 actor type 做 Check；`"default"` fallback ID 改为可配置默认值

## Non-goals

- 不在本阶段处理 `authn.result` 事件的代码生成（Phase 4 随 Keycloak 接入）
- 不修改 Audit Service（`app/audit/service`）的消费/存储逻辑
- 不在本阶段做运行时动态规则加载/热更新
- 不改造 sayhello 为分层完备微服务（仅补足 proto 注解作为验证目标）
- 不修改 `pkg/authn`（已正确使用 `WithClaimsMapper` option 模式，无需去特化）
- 不修改 `pkg/transport/server/middleware/identity.go`（已正确使用 `WithHeaderMapping` option 模式）

## Capabilities

### New Capabilities
- `protoc-gen-servora-audit`: 审计注解代码生成器，读取 proto audit_rule option → 生成 `audit_rules.gen.go`（func 返回 copy），包含 target_id_field 提取 helper
- `audit-codegen-integration`: buf 生成链路集成（`buf.audit.gen.yaml` + `make api`）与 sayhello E2E 验证
- `pkg-despecialization`: pkg 框架包去特化 — actor scope 通用化、scope middleware 可配置化、authz principal 泛化、dead code 清理

### Modified Capabilities
- `audit-proto`: `annotations.proto` 中 `operation` 字段改为 `ResourceMutationType mutation_type` 枚举
- `audit-runtime`: middleware 适配 codegen 输出形式，支持 `func() map[string]Rule` 注入
- `authz-audit-emit`: `protoc-gen-servora-authz` 输出从 `var` 改为 `func()`，`pkg/authz` 适配新签名 + principal 构造泛化 + 多 actor type 支持
- `actor-v2`: 删除业务特化常量/方法/legacy 字段，新增通用 `ScopeFromContext`

## Impact

- **Proto**: `api/protos/servora/audit/v1/annotations.proto` — 字段类型变更（`string` → `enum`）
- **Codegen**: 新增 `cmd/protoc-gen-servora-audit`；修改 `cmd/protoc-gen-servora-authz`
- **Buf 配置**: 新增 `buf.audit.gen.yaml`
- **Makefile**: `make api` 增加 audit gen 步骤
- **pkg/actor**: 删除业务特化常量/方法/legacy 字段，新增通用 context helper
- **pkg/audit**: `middleware.go` — `WithRulesFunc` + `TargetIDFunc`
- **pkg/authz**: `authz.go` — `WithAuthzRulesFunc` + principal 泛化 + 多 actor type（**BREAKING**）
- **pkg/transport/server/middleware**: `scope.go` 可配置化（**BREAKING**）
- **app/iam/service**: 调用 `AuthzRules` → `AuthzRules()`，适配 scope/actor 变更
- **app/sayhello/service**: proto 加注解 + 移除 `grpc.go` 手写 rules
- **api/gen/go**: 重新生成所有 `authz_rules.gen.go` + 新增 `audit_rules.gen.go`
