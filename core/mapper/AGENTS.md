# AGENTS.md - core/mapper/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-22 | Updated: 2026-03-22 -->

## 模块目的

提供类型安全读投影映射运行时，支撑 `ENTITY -> DTO` 转换、生成配置应用、通用 converter 与 Go 侧 post hook。

## 当前文件

- `mapper.go`：泛型 mapper API 主入口
- `copier.go`：`Config`、`Apply` 与 `CopierMapper[DTO, ENTITY]` 读投影辅助
- `converter.go`：转换器定义与实现辅助
- `mapper_test.go`：相关测试

## 当前实现事实

- `mapper.go` 提供类型安全函数式 mapper API
- `copier.go` 承担生成配置的表达、执行和反射读投影
- `CopierMapper` 只负责读投影，不负责 `DTO -> ENTITY` 写入
- 通用 converter 默认随 `NewCopierMapper` 装配，业务 enum / JSON / edge 转换留在 Go 侧显式注册或 post hook

## 边界约束

- 本包负责“如何读投影”，不负责业务 DTO 设计与领域决策
- 不在这里放服务私有的字段语义解释或跨服务编排逻辑
- 不在这里绑定 ent、GORM 或其他 ORM 写入 API

## 常见反模式

- 用 mapper 盲拷贝 `DTO -> ENTITY` 写入存储
- 把 post hook 用成承载复杂业务副作用的扩展点
- 在 converter 中夹带 repository / RPC 调用，破坏纯转换边界

## 测试与使用

```bash
go test ./core/mapper/...
```

## 维护提示

- 若调整 `Config` 结构或内建 converter 语义，需同步检查生成链路输出是否兼容
- post hook / converter 应尽量保持纯函数式，避免引入隐藏副作用
