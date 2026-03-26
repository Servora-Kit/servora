# Transport Runtime Graph 设计文档

## 背景

当前 `servora/transport` 的客户端与服务端虽然都可用，但代码组织模式不一致：

- `server` 以协议目录 + option builder 为主（`http/grpc/sse`）
- `client` 以 factory + conn 逻辑为主（`grpc/http`）

这会导致共享能力（TLS、endpoint advertise、配置归一、错误包装）分散在多处，后续扩展成本上升。

## 目标

1. 统一 client/server 的构建编排模式（Runtime Graph）
2. 保留协议实现与协议中间件的独立维护边界
3. 首期仅支持：
   - server: `http` `grpc` `sse`
   - client: `http` `grpc`
4. 保留扩展接口（插件注册能力），但首期不适配外部组件
5. 破坏性升级可接受，先落地 `servora-example`，再迁移 `iam/platform`

## 非目标

1. 首期不接入外部 `kratos-transport` 组件
2. 不做向后兼容层
3. 不在本次设计中引入新业务协议类型

## 总体架构

新增 `transport/runtime` 编排层，并提取 `transport/shared` 公共能力层。

- `runtime` 只负责：配置归一、插件解析、构建编排、生命周期与清理
- `shared` 只放无协议绑定的横切能力
- `server/*` 与 `client/*` 保持各自协议实现与中间件链

## 目录规划

```text
transport/
  runtime/
    contracts.go
    registry.go
    graph.go
    bootstrap.go
    errors.go
  shared/
    tls/
    endpoint/
    config/
    errors/
  server/
    grpc/
      plugin.go
      builder.go
      middleware.go
    http/
      plugin.go
      builder.go
      middleware.go
    sse/
      plugin.go
      builder.go
  client/
    grpc/
      plugin.go
      builder.go
      middleware.go
    http/
      plugin.go
      builder.go
      middleware.go
```

## 扩展接口（保留未来扩展能力）

`runtime` 提供稳定插件契约：

- `ServerPlugin{ Type(), Build(...) }`
- `ClientPlugin{ Type(), Build(...) }`
- `PluginRegistry{ RegisterServer, RegisterClient, Server, Client }`

约束：

1. 首期内置注册仅包含 `http/grpc/sse`（server）和 `http/grpc`（client）
2. 未注册类型在启动阶段 fail-fast
3. 扩展方仅依赖 `contracts + shared`，不侵入 runtime 核心

## 数据流设计

### 启动阶段

1. `bootstrap` 加载配置
2. `runtime` 执行配置归一与校验
3. 按配置类型从 registry 取 plugin 并构建
4. 输出：`servers + client factories + cleanup`
5. 应用层使用 `kratos.Server(...)`、`kratos.Registrar(...)` 启动

### 请求阶段

- 服务端仅走服务端协议中间件
- 客户端仅走客户端协议中间件
- `shared` 不进入热路径分发，只用于构建期

## 共享能力拆分

1. `shared/tls`
   - 统一客户端/服务端 TLS config 构建入口
2. `shared/endpoint`
   - 统一 `advertise_endpoint/advertise_host` 解析
   - 提供 scheme + secure 标准化
3. `shared/config`
   - timeout/endpoint/tls 默认值与合法性校验
4. `shared/errors`
   - 统一错误分类：`ErrInvalidConfig` `ErrPluginNotFound` `ErrBuildFailed`

## 配置策略

- 首期保持现有配置语义不变
- 由 runtime 统一解析并传入各协议 plugin
- `advertise` 能力继续优先服务 gRPC；HTTP 可作为后续同构扩展

## 测试策略

1. 单元测试
   - runtime: registry / fail-fast / build graph
   - shared: tls / endpoint / config normalize
   - protocol plugin: builder + middleware + endpoint
2. 集成测试（先 `servora-example`）
   - master -> worker: Consul discovery + TLS
   - server: http/grpc/sse 并行装配
   - client: grpc/http 工厂构建与调用
3. 回归测试
   - 迁移 `iam/platform` 后执行 smoke + 关键链路回归

## 迁移顺序

1. 引入 runtime/shared 基础包（不切业务）
2. 迁移 server 三协议为 plugin
3. 迁移 client 两协议为 plugin
4. 接入 `servora-example`（验收基线）
5. 接入 `iam/platform`
6. 清理旧构建路径

## 风险与控制

1. 风险：重构面较大，路径漂移导致启动失败
   - 控制：先 example 冻结验收链路再推广
2. 风险：shared 过度抽象，反而增加耦合
   - 控制：shared 仅放无协议语义逻辑，禁止业务/协议分支
3. 风险：启动编排错误定位困难
   - 控制：统一错误码 + 构建阶段标签（`[client.grpc.build]`）

## 验收标准（DoD）

1. `servora-example` 通过 runtime graph 启动
2. `master -> worker` discovery + TLS 调用成功
3. server/client 协议中间件仍分别维护
4. 新增协议类型时可通过插件注册扩展，不改 runtime 核心

