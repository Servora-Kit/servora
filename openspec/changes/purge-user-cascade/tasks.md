# 任务：PurgeUser 级联删除原子性与僵尸数据

## 第一阶段：执行顺序调整

- [x] **T1: PurgeUser 顺序改为先 DB 再 FGA 再 Redis**
  - 文件：`app/iam/service/internal/biz/user.go`
  - 当前顺序：purgeUserFGA → DeleteUserRefreshTokens → PurgeCascade
  - 改为：PurgeCascade → purgeUserFGA → DeleteUserRefreshTokens
  - PurgeCascade 失败则直接返回，不执行 FGA/Redis
  - FGA 或 Redis 失败时打日志，可选返回错误或部分失败信息
  - 验证：单元测试或手工调用 PurgeUser，确认顺序与错误路径

## 第二阶段：PurgeCascade 删除用户拥有的 Org/Project

- [x] **T2: 在 PurgeCascade 中查出用户为 owner 的 Organization 列表**
  - 文件：`app/iam/service/internal/data/user.go`
  - 在现有事务内：Query OrganizationMember.Where(UserIDEQ(uid), RoleEQ("owner"))，得到 orgID 列表
  - 若无 owner 概念或 role 字段名不同，按实际 schema 调整（目标：该用户「拥有」的 org）

- [x] **T3: 对每个 owned org 在同一事务内执行 cascade**
  - 文件：`app/iam/service/internal/data/user.go`（或复用 organization.go）
  - 对每个 orgID：删除该 org 下 Project 的 ProjectMember → 删除 Project → 删除该 org 的 OrganizationMember → 删除 Organization
  - 若 Organization 下有 Application 且无 DB 外键 CASCADE，需显式删除 Application（参考 organizationRepo.PurgeCascade 或 schema）
  - 保持与现有 `organizationRepo.PurgeCascade(orgID)` 的依赖顺序一致，可抽成共享函数在 tx 内调用

- [x] **T4: 再执行现有 Member + User 删除**
  - 在 T3 之后，同一事务内：OrganizationMember.Where(UserIDEQ)、ProjectMember.Where(UserIDEQ)、User.DeleteOneID
  - 验证：PurgeUser 后 DB 中无该用户的 User、OrganizationMember、ProjectMember，且其拥有的 Organization、Project 也被删除

## 第三阶段：可观测与补偿（可选）

- [x] **T5: PurgeUser 打点/日志**
  - 各步（PurgeCascade 开始/成功、FGA、Redis）打结构化日志或 span，便于排查中断点

- [x] **T6: 按 user_id 清理 FGA/Redis 的补偿脚本或管理接口**
  - 输入：user_id
  - 行为：删除 FGA 中该 user 的 org/project/tenant 相关 tuple；删除 Redis 中该用户的 refresh token
  - 用于 PurgeCascade 成功但 FGA/Redis 失败后的手工修复
