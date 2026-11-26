# AGENTS.md - core/

<!-- Parent: ../AGENTS.md -->

## 目录定位

`core/` 是 Servora 的框架横切协议与平台能力聚合根。

## 准入标准（Admission Gate）

新增成员 MUST 同时满足：被多个 capability 复用、有清晰协议、且不绑定业务语义。单一 capability 的内部工具归属 capability 子包；不得创建 `util`/`helper`/`common` 杂物包。

## 当前成员

- `bootstrap/` 启动装配与生命周期
- `config/` 配置加载
- `registry/` 服务注册与发现

新增 core 包必须在 PR 描述中逐条说明准入标准。

## 验证

```bash
go test ./core/...
go test -short ./...
```

## 反模式

- `core/util/`、`core/helper/`、`core/common/`：无主题集合包（`pkg/utils` 反模式同构）
- 单 capability 使用的工具被「升根」进 core
- 业务语义被「框架化」放进 core

## 维护提示

- 新成员加入须先在 PR 描述里答辩准入标准
- `bootstrap/` / `config/` / `registry/` 是「平台能力」类成员，与「框架横切协议」共享同一聚合根，但准入闸同样适用
