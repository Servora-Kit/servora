# 设计：PurgeUser 级联删除原子性与僵尸数据

## 现状

### 当前 PurgeUser 流程（`internal/biz/user.go`）

```
1. purgeUserFGA(ctx, user.ID)     → 删除该用户在 FGA 中的 tuple（org/project 角色 + tenant admin）
2. authnRepo.DeleteUserRefreshTokens(ctx, user.ID)  → Redis 删 refresh token
3. repo.PurgeCascade(ctx, user.ID)  → DB 事务内：删 OrganizationMember、ProjectMember、User
```

顺序：**FGA → Redis → DB**。FGA/Redis 失败仅打 Warnf，不阻断后续；PurgeCascade 失败则返回错误。

### PurgeCascade 当前逻辑（`internal/data/user.go`）

在 `RunInEntTx` 内：

1. `OrganizationMember.Delete().Where(UserIDEQ(uid))`
2. `ProjectMember.Delete().Where(UserIDEQ(uid))`
3. `User.DeleteOneID(uid)`

**未做**：删除该用户作为 **owner** 的 Organization，以及这些 Organization 下的 Project（及 ProjectMember）。现有 `organizationRepo.PurgeCascade(orgID)` 已实现「删 ProjectMember → Project → OrganizationMember → Organization」的依赖顺序，可复用。

---

## 决策

### D1：PurgeUser 跨系统顺序

**选择：先 DB（PurgeCascade）再 FGA 再 Redis。**

理由：

- 先删 DB 后，用户已不可登录、无业务数据；即使后续 FGA/Redis 失败，残留的 FGA tuple / Redis key 仅造成「僵尸授权」，可通过补偿脚本按 user_id 清理。
- 若先删 FGA/Redis 再 DB，DB 失败时 FGA 已无该用户 tuple，难以仅凭 FGA 做补偿，且用户仍存在 DB 中，状态更混乱。
- Redis 放最后：PurgeCascade 成功后用户已不存在，最后清 Redis 失败可重试或补偿。

实施要点：

- 调整 `UserUsecase.PurgeUser` 顺序为：`PurgeCascade` → `purgeUserFGA` → `DeleteUserRefreshTokens`。
- 若 `PurgeCascade` 失败，直接返回，不写 FGA/Redis。
- 若 `purgeUserFGA` 或 `DeleteUserRefreshTokens` 失败，记录日志并可选返回部分失败错误；建议至少打点（如 span/结构化日志）便于后续补偿。

### D2：PurgeCascade 内删除用户拥有的 Organization、Project

**选择：在同一事务内，先查出该用户作为 owner 的 Organization 列表，对每个 org 执行与 `organizationRepo.PurgeCascade(orgID)` 相同的删除顺序（ProjectMember → Project → OrganizationMember → Organization），再执行现有 OrganizationMember、ProjectMember、User 删除。**

理由：

- 保持「单事务」原子：要么全部删掉（含用户拥有的 org/project），要么全部回滚。
- 复用现有 `organizationRepo.PurgeCascade` 的依赖顺序逻辑；可在 data 层抽成「按 orgID 在给定 tx 内执行 cascade」供 userRepo 调用，避免重复代码。

依赖顺序（与 organization.go 一致）：

1. 对每个「该用户为 owner 的 org」：  
   - 该 org 下所有 Project 的 ProjectMember → Project → 该 org 的 OrganizationMember → Organization  
2. 全局：OrganizationMember.Where(UserIDEQ)、ProjectMember.Where(UserIDEQ)、User。

注意：需先根据 **当前 DB 中 OrganizationMember 的 role=owner** 判定「用户拥有的 org」，再执行上述顺序。Application 若挂在 Organization 下且 schema 有外键 CASCADE，由 DB 级联；否则需在 PurgeCascade 中显式删除 Application（参考 organizationRepo 是否已包含 Application 删除，若未包含则补）。

### D3：补偿与可观测性（可选）

- **打点/日志**：PurgeUser 各步（PurgeCascade 开始/成功、FGA 开始/成功、Redis 开始/成功）打日志或 span，便于排查中断点。
- **补偿**：提供脚本或管理接口，按 `user_id` 删除 FGA 中该 user 的 tuple 并清理 Redis 中该用户的 refresh token；用于 PurgeCascade 成功但 FGA/Redis 失败后的手工修复。

---

## 涉及文件

| 文件 | 变更 |
|------|------|
| `app/iam/service/internal/biz/user.go` | PurgeUser 顺序改为 PurgeCascade → purgeUserFGA → DeleteUserRefreshTokens；错误处理与日志 |
| `app/iam/service/internal/data/user.go` | PurgeCascade 内增加：查用户为 owner 的 org 列表 → 对每个 org 执行 cascade（ProjectMember→Project→OrganizationMember→Organization）→ 再执行现有 Member/User 删除 |
| `app/iam/service/internal/data/organization.go` | 可选：抽出「在给定 tx 内对单个 orgID 执行 PurgeCascade」供 userRepo 调用 |
| 脚本/管理接口（可选） | 按 user_id 清理 FGA tuple + Redis refresh token |

---

## 参考

- TODO.md：级联删除原子性与僵尸数据
- docs/design/multi-tenancy-prerequisites.md：§6 PurgeUser 级联删除
