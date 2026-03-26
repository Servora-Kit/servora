# AGENTS.md - core/actor/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-22 | Updated: 2026-03-22 -->

## 模块目的

定义请求上下文中的 Actor v2 抽象，统一表达 `User`、`Service`、`System`、`Anonymous` 四类身份及其 `Subject`、`Roles`、`Scopes`、`Attrs` 等访问属性。

## 当前文件

- `actor.go`：`Actor` 接口与共享字段访问约定
- `user.go`：用户 Actor
- `service.go`：服务 Actor
- `system.go`：系统 Actor
- `anonymous.go`：匿名 Actor
- `context.go`：Actor 的 context 注入与提取

## 当前实现事实

- `Actor` 接口同时暴露 `ID`、`Type`、`DisplayName`、`Email`、`Subject`、`ClientID`、`Realm`、`Roles`、`Scopes`、`Attrs`
- `Scope(key string)` 用于按 key 读取 scope 值；这里的 scope 指 Actor 语义上的访问范围，不等同于 Go `context` 的作用域概念
- 该包本身不做鉴权决策，只承载身份表达与跨层传递
- `context.go` 是 transport / middleware 与业务层之间传递 Actor 的桥梁

## 边界约束

- 这里只定义身份模型与上下文传递，不负责 token 解析、claims 校验或权限判断
- 不在本包引入 IAM 业务概念（组织、项目、成员关系等）
- 不在本包耦合 HTTP / gRPC 细节；协议层适配应留在 `transport` 或上层 middleware

## 常见反模式

- 把 JWT claims 解析逻辑直接塞进 `core/actor`
- 把 OpenFGA、角色授权或资源判定逻辑塞进 Actor 类型
- 混淆 OAuth scopes 与请求上下文 / 代码块中的“scope”概念

## 测试与使用

```bash
go test ./core/actor/...
```

## 维护提示

- 若新增 Actor 类型，需同步检查 context 注入/提取与所有调用方的类型分支
- 若调整 `Actor` 接口字段，优先确认 `security/authn`、`security/authz` 与服务内 middleware 的兼容性
