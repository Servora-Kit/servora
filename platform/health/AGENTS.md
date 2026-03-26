# AGENTS.md - platform/health/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-22 | Updated: 2026-03-22 -->

## 模块目的

提供健康检查运行时能力，统一组织 liveness / readiness 检查器、超时控制与 HTTP handler 输出。

## 当前文件

- `health.go`：`Checker`、`Pinger`、`Handler` 等核心抽象
- `builder.go`：健康检查构造辅助
- `defaults.go`：默认检查行为与常量
- `health_test.go`：健康检查测试

## 当前实现事实

- liveness 默认始终返回 200，用于表达进程“活着”
- readiness 会执行已注册的检查器，按结果决定服务是否就绪
- 默认超时为 3 秒，避免检查无限阻塞
- 包内关注的是检查运行时与聚合，不是业务健康指标本身

## 边界约束

- 本包只负责健康检查编排，不负责业务修复、熔断或告警派发
- 不把领域状态判断硬编码进默认检查器；具体依赖探测由调用方注入
- 不在这里混入 transport 以外的监控面板、Prometheus 或 tracing 逻辑

## 常见反模式

- 把 readiness 做成永远返回 200，失去流量保护意义
- 在检查器里执行耗时业务逻辑或带副作用的写操作
- 混淆 liveness 与 readiness，用同一套语义处理所有探针

## 测试与使用

```bash
go test ./platform/health/...
```

## 维护提示

- 若新增默认检查项，优先保证其无副作用、可超时、可组合
- 若调整超时或返回语义，需同步检查 K8s / Compose 探针配置是否匹配
