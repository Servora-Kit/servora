# AGENTS.md - core/mapper/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-22 | Updated: 2026-03-22 -->

## 模块目的

提供类型安全映射运行时，统一组织 mapper、plan、preset、hook 与 converter，支撑对象间转换与生成计划执行。

## 当前文件

- `mapper.go`：泛型 mapper API 主入口
- `plan.go`：`MapperPlan`、内建 converter kind、校验与 `ApplyPlan`
- `preset.go`：预设映射能力
- `hook.go`：映射 hook 定义
- `converter.go`：转换器定义与实现辅助
- `copier_proto.go`：proto 复制辅助
- `mapper_test.go`：相关测试

## 当前实现事实

- `mapper.go` 提供类型安全 mapper 运行时 API
- `plan.go` 承担声明式映射计划的表达、校验与执行
- preset / hook / converter 各自分层，避免把所有转换规则塞进单个入口
- 该包既服务手写映射，也服务 protoc / 生成计划落地后的运行时执行

## 边界约束

- 本包负责“如何映射”，不负责业务 DTO 设计与领域决策
- 不在这里放服务私有的字段语义解释或跨服务编排逻辑
- 不把 mapper runtime 与 codegen 结果手工复制粘贴混用

## 常见反模式

- 在业务代码里重复手写已可由 `MapperPlan` 表达的转换
- 把 hook 用成承载复杂业务副作用的扩展点
- 在 converter 中夹带 repository / RPC 调用，破坏纯转换边界

## 测试与使用

```bash
go test ./core/mapper/...
```

## 维护提示

- 若调整 `MapperPlan` 结构或内建 converter 语义，需同步检查生成链路输出是否兼容
- hook / converter 应尽量保持纯函数式，避免引入隐藏副作用
