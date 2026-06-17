# AGENTS.md - contrib/

<!-- Parent: ../AGENTS.md -->

## 目录定位

`contrib/` 是 Servora 的可选生态集成层。这里放第三方系统 client 接线、连接生命周期、配置映射、日志/tracing、健康检查和可选 adapter。

旧 infra 根目录不再作为新增包路径存在；外部系统接线进入 `contrib/`。

## 当前布局

```text
contrib/
├── db/
│   ├── entgo/
│   ├── gorm/
│   ├── clickhouse/
│   └── redis/
├── cache/
│   └── redis/
├── kafka/
└── k8s/
```

Mail 只作为 proto 便利配置存在于 `servora/api/protos/servora/contrib/mail/v1/config.proto`，不创建 `contrib/mail` Go runtime 包。

## 准入标准

`contrib/` 成员 MUST 是可选生态集成或第三方系统接线，提供：

- client 工厂与配置映射
- 连接管理、关闭和生命周期辅助
- 日志、tracing、健康检查
- 后端通用 helper
- 明确的可选 adapter

`contrib/` 成员 MUST NOT 混入业务领域语义：

- 不写业务 entity 的 repository
- 不写业务缓存 key 设计或失效策略
- 不写 audit 事件 schema、retention、table DDL
- 不写 notification、模板、收件箱或消息中心语义

## 分层规则

- 基础封装放在对应基础包，例如 `contrib/db/entgo`、`contrib/db/redis`、`contrib/kafka`。
- CRUD adapter 只能放在对应 `crud` 子目录，例如 `contrib/db/entgo/crud`、`contrib/db/gorm/crud`。
- 基础封装不得 import CRUD runtime。
- Redis 基础 client、lock、KV helper 属于 `contrib/db/redis`；cache 策略和 cache backend 属于 `contrib/cache` 或 `contrib/cache/redis`。
- Kafka 只负责 franz-go native client 接线，不承载 audit 语义。
- K8s 只负责 Kubernetes API client 基础接线和环境辅助。

## 维护提示

- 新增第三方系统接线前先判断所属领域，优先复用 `db/`、`cache/`、顶层平台/消息目录。
- 新增 Go 包时同步检查 `servora/AGENTS.md` 的 scope 示例。
- 修改公共 proto 配置时必须同步执行 `make gen.fresh` 和 `make gen`。
