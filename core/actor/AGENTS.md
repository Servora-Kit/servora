# AGENTS.md - core/actor/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-22 | Updated: 2026-05-10 -->

## 模块目的

定义请求上下文中的 Actor 抽象，统一表达 `User`、`Service`、`System`、`Anonymous` 四类调用源拓扑身份。Actor 仅承载调用源拓扑通用语，不承载 OAuth/OIDC 协议特定字段。

## 当前文件

- `actor.go`：`Actor` 接口三件套（`ID/Type/DisplayName`）与四个 Type 常量
- `user.go`：用户 Actor
- `service.go`：服务 Actor
- `system.go`：系统 Actor
- `anonymous.go`：匿名 Actor
- `context.go`：Actor 的 context 注入与提取（`NewContext` / `From` / `MustFrom`）

## 当前实现事实

- `Actor` 接口仅暴露 `ID() string`、`Type() Type`、`DisplayName() string` 三件套
- 四个 Type 常量保持 `TypeUser`/`TypeService`/`TypeSystem`/`TypeAnonymous`，业务可自扩
- 协议特定字段（`Email/Subject/ClientID/Realm/Roles/Scopes`）与开放扩展袋（`Attrs`）不在 `Actor` 接口暴露；由业务自定义 ctx 信道（如业务自家 IAM 包提供 `WithUserInfo` / `UserInfoFromContext`）承载，servora 主仓不预设 OIDC ctx 信道
- ctx accessor 命名不含 `Context` 后缀：`From(ctx) (Actor, bool)` 与 `MustFrom(ctx) Actor`；setter 保留 `NewContext(ctx, a)`
- 该包本身不做鉴权决策，只承载身份表达与跨层传递
- `context.go` 是 transport / middleware 与业务层之间传递 Actor 的桥梁

## 边界约束

- 这里只定义身份模型与上下文传递，不负责 token 解析、claims 校验或权限判断
- 不在本包引入 IAM 业务概念（组织、项目、成员关系等）
- 不在本包耦合 HTTP / gRPC 细节；协议层适配应留在 `transport` 或上层 middleware
- 不在 `Actor` 接口暴露 OAuth/OIDC 协议字段；新增方法前先确认是否属于"调用源拓扑通用语"

## 常见反模式

- 把 JWT claims 解析逻辑直接塞进 `core/actor`
- 把 OpenFGA、角色授权或资源判定逻辑塞进 Actor 类型
- 通过具体类型断言（`a.(*UserActor).Email()` 之类）绕过接口暴露 OIDC 字段——OIDC 字段一律走业务自定义 ctx 信道
- ctx accessor 命名加 `Context` 后缀

## 测试与使用

```bash
GOWORK=off go test ./core/actor/...
```

## 维护提示

- 若新增 Actor 类型，需同步检查 context 注入/提取与所有调用方的类型分支
- 若调整 `Actor` 接口字段，优先确认 `security/authn`、`security/authz`、`obs/audit` 与服务内 middleware 的兼容性
- OIDC 字段补齐 / `AuditActor` 4 OIDC 字段填充由业务侧 IAM 适配层（如 servora-platform/iam）的 Recorder wrapper 负责
