## 1. pkg/actor 去特化

- [x] 1.1 `pkg/actor/user.go`：删除 `ScopeKeyTenantID` / `ScopeKeyOrganizationID` / `ScopeKeyProjectID` 常量
- [x] 1.2 `pkg/actor/user.go`：删除 6 个便捷方法 `TenantID()` / `SetTenantID()` / `OrganizationID()` / `SetOrganizationID()` / `ProjectID()` / `SetProjectID()`
- [x] 1.3 `pkg/actor/user.go`：删除 `UserActorParams.Metadata` 字段、`UserActor.metadata` 字段、`Metadata()` 和 `Meta()` 方法
- [x] 1.4 `pkg/actor/context.go`：删除 `TenantIDFromContext` / `OrganizationIDFromContext` / `ProjectIDFromContext`，新增通用 `ScopeFromContext(ctx, key) (string, bool)`
- [x] 1.5 `pkg/actor/system.go`：`ID()` 改为返回构造时传入的 `id`（不再自动拼接 `"system:"` 前缀），`NewSystemActor` 签名调整
- [x] 1.6 确认 `pkg/actor` 编译通过

## 2. pkg/transport/server/middleware/scope.go 可配置化

- [x] 2.1 定义 `ScopeBinding` struct（`Header` / `ScopeKey` / `Validate`）
- [x] 2.2 重写 `ScopeFromHeaders(bindings ...ScopeBinding)` — 遍历 bindings 而非硬编码 3 个 header
- [x] 2.3 删除 `TenantIDHeader` / `OrganizationIDHeader` / `ProjectIDHeader` 常量
- [x] 2.4 更新 `scope_test.go` 适配新 API
- [x] 2.5 确认 `pkg/transport` 编译通过

## 3. pkg/authz 去特化

- [x] 3.1 `pkg/authz/authz.go`：`principal := "user:" + userID` → `principal := string(a.Type()) + ":" + a.ID()`
- [x] 3.2 `pkg/authz/authz.go`：移除 `a.Type() != actor.TypeUser` 硬判断 → 改为拒绝 `anonymous`、允许其他 type 通过
- [x] 3.3 `pkg/authz/authz.go`：新增 `WithDefaultObjectID(id string)` option，默认值 `"default"`
- [x] 3.4 `pkg/authz/authz.go`：新增 `WithAuthzRulesFunc(fn func() map[string]AuthzRule)` option，保留 `WithAuthzRules` 向后兼容
- [x] 3.5 更新 `pkg/authz/authz_test.go` 适配上述变更
- [x] 3.6 确认 `pkg/authz` 编译通过

## 4. 调用方适配（actor/scope/authz 变更）

- [x] 4.1 `app/iam/service`：适配 actor scope 变更（定义本地 scope key 常量，替换 `actor.ScopeKey*` 引用，替换 `actor.TenantIDFromContext` → `actor.ScopeFromContext`）
- [x] 4.2 `app/iam/service`：适配 `NewSystemActor` 签名变更（如有使用）
- [x] 4.3 `app/iam/service`：scope middleware 调用方式改为 `ScopeFromHeaders(bindings...)`
- [x] 4.4 `app/sayhello/service`：同上适配（如有相关引用）
- [x] 4.5 确认所有 Go workspace module 编译通过：`go build ./...`

## 5. Proto 变更与重新生成

- [x] 5.1 修改 `api/protos/servora/audit/v1/annotations.proto`：`string operation = 3` → `ResourceMutationType mutation_type = 3`，更新注释和使用示例
- [x] 5.2 运行 `make api` 重新生成 proto Go 代码，确认 `annotations.pb.go` 中 `AuditRule.MutationType` 字段类型正确

## 6. protoc-gen-servora-authz 改造（BREAKING）

- [x] 6.1 修改 `cmd/protoc-gen-servora-authz/main.go` 中 `generateFile`：`var AuthzRules` → unexported `var _authzRules` + exported `func AuthzRules() map[string]authz.AuthzRule`（返回 copy）
- [x] 6.2 运行 `make api` 重新生成 `authz_rules.gen.go`，确认输出从 `var` 变为 `func`
- [x] 6.3 修改 `app/iam/service/internal/server/grpc.go` 和 `http.go`：`iampb.AuthzRules` → `iampb.AuthzRules()`
- [x] 6.4 确认 IAM 和 sayhello 编译通过

## 7. pkg/audit middleware 适配

- [x] 7.1 `pkg/audit/event.go`：在 `Rule` struct 中新增 `TargetIDFunc func(req, resp any) string` 字段
- [x] 7.2 `pkg/audit/middleware.go`：新增 `WithRulesFunc(fn func() map[string]Rule)` option
- [x] 7.3 `pkg/audit/middleware.go`：handler 返回后如果 rule 有 `TargetIDFunc`，调用它并将结果设入 `TargetInfo.ID`
- [x] 7.4 确认 `pkg/audit` 编译通过，现有 `WithRules` 仍可用

## 8. protoc-gen-servora-audit 实现

- [x] 8.1 创建 `cmd/protoc-gen-servora-audit/main.go`：基本框架（参考 authz 插件结构），解析 `audit_rule` method option
- [x] 8.2 实现规则提取逻辑：遍历 proto files → services → methods，收集 `AuditRule` 注解
- [x] 8.3 实现服务别名扩展（与 authz 插件一致的 `svcAliases` 逻辑）
- [x] 8.4 实现 `generateFile`：输出 `_auditRules` var + `AuditRules()` func（copy 返回），import `pkg/audit`
- [x] 8.5 实现 `target_id_field` 代码生成：为含 `target_id_field` 的规则生成 `_extract<Method>TargetID(req, resp any) string` 函数
- [x] 8.6 `target_id_field` 支持 `req.<field>` 和 `resp.<field>` 两种前缀，使用 proto message 的 `Get<Field>()` 方法
- [x] 8.7 安装插件到 `$GOPATH/bin`：`go install ./cmd/protoc-gen-servora-audit`

## 9. Buf 生成链路集成

- [x] 9.1 创建 `buf.audit.gen.yaml`：配置 `protoc-gen-servora-audit` 插件，`out: api/gen/go`，`opt: paths=source_relative`
- [x] 9.2 修改 `Makefile`：`make api` 增加 `buf generate --template buf.audit.gen.yaml` 步骤（在 authz 之后）
- [x] 9.3 运行 `make api` 验证全链路生成无报错

## 10. sayhello 验证与集成

- [x] 10.1 修改 `app/sayhello/service/api/protos/servora/sayhello/service/v1/sayhello.proto`：import annotations.proto，为 `Hello` RPC 添加 `audit_rule` 注解
- [x] 10.2 运行 `make api`，确认 `audit_rules.gen.go` 在 sayhello Go 包中生成
- [x] 10.3 修改 `app/sayhello/service/internal/server/grpc.go`：将手写 `audit.WithRules(map[string]audit.Rule{...})` 替换为 `audit.WithRulesFunc(sayhellopb.AuditRules)` 或 `audit.WithRules(sayhellopb.AuditRules())`
- [x] 10.4 确认 sayhello 编译通过

## 11. 文档与收尾

- [x] 11.1 更新 `docs/plans/2026-03-20-keycloak-openfga-audit-design.md`：Phase 3 状态更新、新增约束（去特化相关）
- [x] 11.2 E2E 验证：`make compose.dev` → 调用 sayhello Hello → 确认审计事件落库 → `GET /v1/audit/events` 可见
- [x] 11.3 提交代码（按原子性分组提交）
