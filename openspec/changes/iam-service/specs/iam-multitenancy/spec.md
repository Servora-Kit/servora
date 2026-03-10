# IAM Multitenancy Specification

## ADDED Requirements

### Requirement: The system MUST enforce a three-tier tenant model with fixed Platform

系统 MUST 支持三层租户模型（Platform → Tenant → Workspace），V1 固定 Platform 为 `platform:root`（id=1），用户只需管理 Tenant 和 Workspace。

#### Scenario: Platform root is pre-initialized

- **WHEN** 系统首次启动
- **THEN** 数据库中必须存在 Platform 记录（id=1, slug="root", type="system"）

#### Scenario: All tenants belong to platform root

- **WHEN** 用户创建 Tenant
- **THEN** Tenant 的 `platform_id` 自动设置为 1

#### Scenario: Platform management API is not exposed

- **WHEN** 用户尝试访问 Platform CRUD API
- **THEN** 系统返回错误 "Platform management is not available in V1"，HTTP 状态码 404

### Requirement: The system MUST support tenant creation and management

系统 MUST 支持 Tenant 的创建、查询、更新和软删除。

#### Scenario: Create tenant

- **WHEN** 用户创建 Tenant 并提供 name 和 slug
- **THEN** 系统创建 Tenant 记录（platform_id=1）、创建 OpenFGA 关系元组（user is owner of tenant）、返回 Tenant 信息

#### Scenario: Create tenant with duplicate slug

- **WHEN** 用户使用已存在的 slug 创建 Tenant
- **THEN** 系统返回错误 "Tenant slug already exists"，HTTP 状态码 409

#### Scenario: Update tenant information

- **WHEN** Tenant owner 更新 Tenant 的 name 或 description
- **THEN** 系统更新 Tenant 记录、返回更新后的信息

#### Scenario: Soft delete tenant

- **WHEN** Tenant owner 删除 Tenant
- **THEN** 系统设置 `deleted_at` 字段、级联软删除所有 Workspace、保留 OpenFGA 关系元组用于恢复但必须禁止继续访问

### Requirement: The system MUST support workspace creation and management

系统 MUST 支持 Workspace 的创建、查询、更新和软删除。

#### Scenario: Create workspace

- **WHEN** Tenant member 创建 Workspace 并提供 name 和 slug
- **THEN** 系统创建 Workspace 记录、创建 OpenFGA 关系元组（user is owner of workspace）、返回 Workspace 信息

#### Scenario: Create workspace with duplicate slug in same tenant

- **WHEN** 用户在同一 Tenant 下使用已存在的 slug 创建 Workspace
- **THEN** 系统返回错误 "Workspace slug already exists in this tenant"，HTTP 状态码 409

#### Scenario: Create workspace in different tenants with same slug

- **WHEN** 用户在不同 Tenant 下使用相同的 slug 创建 Workspace
- **THEN** 系统允许创建（slug 在 Tenant 内唯一）

#### Scenario: Update workspace information

- **WHEN** Workspace owner 更新 Workspace 的 name 或 description
- **THEN** 系统更新 Workspace 记录、返回更新后的信息

#### Scenario: Soft delete workspace

- **WHEN** Workspace owner 删除 Workspace
- **THEN** 系统设置 `deleted_at` 字段、保留 OpenFGA 关系元组用于恢复但必须禁止继续访问

### Requirement: The system MUST support tenant member management

系统 MUST 支持 Tenant 成员的邀请、查询和移除。

#### Scenario: Invite user to tenant

- **WHEN** Tenant admin 邀请用户加入 Tenant 并指定角色（owner/admin/member）
- **THEN** 系统创建 TenantMember 记录、创建 OpenFGA 关系元组、发送邀请通知

#### Scenario: List tenant members

- **WHEN** Tenant member 查询 Tenant 成员列表
- **THEN** 系统返回所有成员及其角色

#### Scenario: Remove user from tenant

- **WHEN** Tenant admin 移除成员
- **THEN** 系统删除 TenantMember 记录、删除 OpenFGA 关系元组、级联移除该用户在所有 Workspace 中的成员关系

#### Scenario: Prevent removing last owner

- **WHEN** Tenant admin 尝试移除最后一个 owner
- **THEN** 系统返回错误 "Cannot remove the last owner"，HTTP 状态码 400

### Requirement: The system MUST support workspace member management

系统 MUST 支持 Workspace 成员的邀请、查询和移除。

#### Scenario: Invite user to workspace

- **WHEN** Workspace admin 邀请用户加入 Workspace 并指定角色（owner/admin/member/viewer）
- **THEN** 系统创建 WorkspaceMember 记录、创建 OpenFGA 关系元组、发送邀请通知

#### Scenario: List workspace members

- **WHEN** Workspace member 查询 Workspace 成员列表
- **THEN** 系统返回所有成员及其角色

#### Scenario: Remove user from workspace

- **WHEN** Workspace admin 移除成员
- **THEN** 系统删除 WorkspaceMember 记录、删除 OpenFGA 关系元组

#### Scenario: Prevent removing last owner

- **WHEN** Workspace admin 尝试移除最后一个 owner
- **THEN** 系统返回错误 "Cannot remove the last owner"，HTTP 状态码 400

### Requirement: The system MUST propagate tenant context

系统 MUST 在 HTTP 和 gRPC 请求中传播租户上下文（tenant_id, workspace_id）。

#### Scenario: Extract tenant context from JWT

- **WHEN** 用户请求 API 并提供 JWT Token
- **THEN** 中间件从 Token 中提取 `tenant_id` 和 `workspace_id`、存入请求上下文

#### Scenario: Tenant isolation in data queries

- **WHEN** 用户查询资源列表
- **THEN** 系统自动添加 `tenant_id` 过滤条件、只返回当前 Tenant 的资源

#### Scenario: Cross-tenant access is blocked

- **WHEN** 用户尝试访问其他 Tenant 的资源
- **THEN** 系统返回错误 "Resource not found"，HTTP 状态码 404

### Requirement: The system MUST create a default workspace on registration

系统 MUST 在用户注册时自动创建 Tenant 和默认 Workspace。

#### Scenario: Registration creates tenant and workspace

- **WHEN** 用户完成注册
- **THEN** 系统创建 Tenant（name="{username}'s Tenant", slug="{username}-tenant"）、创建默认 Workspace（name="Default", slug="default"）、将用户设置为 owner

#### Scenario: Default workspace is selected on login

- **WHEN** 用户登录
- **THEN** 系统签发的 JWT Token 包含默认 Workspace 的 `workspace_id`
