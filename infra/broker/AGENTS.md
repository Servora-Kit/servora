# AGENTS.md - infra/broker/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-22 | Updated: 2026-03-22 -->

## 模块目的

定义消息代理抽象层，统一 `Broker`、`Message`、发布订阅与连接生命周期接口，供具体实现复用。

## 当前结构

```text
infra/broker/
├── broker.go
├── message.go
├── options.go
└── kafka/
```

## 当前实现事实

- `broker.go` 定义 `Broker` 抽象，覆盖 connect / disconnect / publish / subscribe 主能力
- `message.go` 承载消息结构
- `options.go` 提供构造和行为选项
- `kafka/` 是 franz-go 的具体实现子树；本级目录只负责抽象，不直接等同于 Kafka

## 边界约束

- 本目录只放通用 broker 抽象与共享模型，不放具体后端驱动实现细节
- Kafka 专有配置、日志、producer / consumer 行为应留在 `infra/broker/kafka`
- 不在这里承载业务事件 schema 或消费重试编排

## 常见反模式

- 在抽象层泄漏 Kafka 专属概念，导致接口失去通用性
- 把业务事件常量和 broker 基础设施放在同一层
- 将连接管理、副作用初始化散落到调用方而不是通过统一接口表达

## 测试与使用

```bash
go test ./infra/broker/...
go test ./infra/broker/kafka/...
```

## 维护提示

- 若扩展新的 broker 后端，优先复用本级接口而不是绕过它另起一套抽象
- 若修改 `Broker` 接口，需同步评估 `kafka/` 与所有调用方的兼容成本
