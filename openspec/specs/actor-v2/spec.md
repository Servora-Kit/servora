# Spec: actor-v2

## Purpose

Defines requirements for the `actor-v2` capability.

## Requirements

### Requirement: Actor interface provides identity-level fields

`Actor` interface SHALL expose the following methods beyond the existing `ID()`, `Type()`, `DisplayName()`:
- `Subject() string` — 外部 IdP subject identifier
- `ClientID() string` — OAuth2 client identifier
- `Realm() string` — IdP realm / tenant namespace
- `Email() string` — 邮箱地址
- `Roles() []string` — 角色列表
- `Scopes() []string` — OAuth2 scope 列表
- `Attrs() map[string]string` — 扩展属性 bag

现有的 `Scope(key string) string` 方法 SHALL 保留，用于请求级维度（由调用方定义 key，框架不预设具体 key）。

#### Scenario: UserActor carries all identity fields

- **WHEN** a `UserActor` is constructed with id, displayName, email, subject, clientID, realm, roles, scopes, and attrs
- **THEN** all getter methods SHALL return the corresponding values

#### Scenario: Missing optional fields return zero values

- **WHEN** a `UserActor` is constructed with only id and type
- **THEN** `Email()` SHALL return `""`, `Roles()` SHALL return `nil` or empty slice, `Attrs()` SHALL return empty map

#### Scenario: No business-specific scope convenience methods

- **WHEN** `UserActor` is inspected
- **THEN** it SHALL NOT expose `TenantID()`, `OrganizationID()`, `ProjectID()` or their setters — only generic `Scope(key)` / `SetScope(key, val)`

#### Scenario: No Metadata legacy field

- **WHEN** `UserActorParams` is inspected
- **THEN** it SHALL NOT have a `Metadata` field — use `Attrs` instead

### Requirement: Existing callers compile after migration

All existing code that creates `UserActor` or consumes `Actor` interface SHALL be updated to compile with the interface changes. This includes `pkg/authn`, `pkg/transport/server/middleware`, `app/iam/service`, `app/sayhello/service`.

#### Scenario: Full project compiles after actor changes

- **WHEN** `go build ./...` is run across all workspace modules
- **THEN** compilation SHALL succeed with zero errors

## REMOVED Requirements

### Requirement: Actor scope key constants for tenant/org/project
**Reason**: Business-specific scope key constants (`ScopeKeyTenantID`, `ScopeKeyOrganizationID`, `ScopeKeyProjectID`) violate the pkg despecialization principle. The generic `Scope(key)` API is sufficient.
**Migration**: Callers define their own scope key constants and use `actor.Scope("tenant_id")` directly.
