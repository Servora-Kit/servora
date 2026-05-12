# AGENTS.md - core/

<!-- Parent: ../AGENTS.md -->

## 目录定位

`core/` 是 servora 的「框架横切协议 + 平台能力」聚合根。它收纳两类东西：

- **框架级横切协议**：被多个 capability 共用、有清晰契约的工具（例如 `pagination` 分页协议、`mapper` proto ↔ entity 映射协议）
- **平台能力**：服务整体生命周期与装配的基础设施（`bootstrap` 启动装配、`config` 配置加载、`registry` 服务注册与发现）

## 准入标准（Admission Gate）

新增成员 MUST **同时**满足以下三项：

1. **框架级横切**——被多个 capability 使用，不是单一 capability 的内部工具
2. **有清晰协议**——包名能在 godoc 上一句话讲清契约（例如 "分页协议"、"proto ↔ entity 映射协议"），不是无主题的杂烩
3. **无业务语义**——不绑定任何业务领域（不出现「用户管理」「订单流程」之类语义）

**新增 PR MUST 在描述里逐条 verify 三项准入标准。** 任一项不满足时拒绝 PR：

- 单 capability 使用 → 归到该 capability 的内部子包
- 无清晰主题（"util" / "helper" / "common"）→ 拆成清晰主题包或归并到现有 capability
- 含业务语义 → 归到对应业务层而非框架核心

## 当前成员

- `bootstrap/` 启动装配与生命周期
- `config/` 配置加载（etcd / nacos / consul）
- `registry/` 服务注册与发现（etcd / nacos / consul / kubernetes）
- `mapper/` proto ↔ entity 映射协议与 protoc 插件运行时
- `pagination/` 分页协议工具

## 反模式

- `core/util/`、`core/helper/`、`core/common/`：无主题集合包（`pkg/utils` 反模式同构）
- 单 capability 使用的工具被「升根」进 core
- 业务语义被「框架化」放进 core

## 维护提示

- 新成员加入须先在 PR 描述里答辩准入标准
- 现有 `mapper/` 与 `pagination/` 是范例：包名一句话讲清，跨多个 capability 使用，无业务语义
- `bootstrap/` / `config/` / `registry/` 是「平台能力」类成员，与「框架横切协议」共享同一聚合根，但准入闸同样适用
