# AGENTS.md - obs/logging/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-15 | Updated: 2026-03-20 -->

## 模块目的

基于 Zap 适配 Kratos 日志接口，并提供 GORM / Ent 日志桥接与结构化字段辅助能力。

## 当前文件

- `log.go`：主日志实现、`ZapLogger`、`New`、`For`、`With`、`WithModule`、`WithField`
- `gorm_log.go`：GORM 日志适配
- `ent_log.go`：Ent 调试日志桥接

## API 速览

| 函数/方法 | 说明 |
|---|---|
| `New(app *conf.App) *ZapLogger` | 从 proto 配置创建 logger，nil-safe |
| `For(l, module) *Helper` | 一行创建带 module 的 Helper（最常用） |
| `With(l, args...) Logger` | 添加字段；支持 string 快捷 module 或 Option 风格 |
| `WithModule(m) Option` | 添加 module 字段的 Option |
| `WithField(k, v) Option` | 添加任意字段的 Option |
| `NewHelper(l, opts...) *Helper` | 保留，供需要多个 Option 的场景 |
| `(*ZapLogger).Zap() *zap.Logger` | 暴露底层 zap（供 franz-go kzap、GORM bridge 等） |
| `(*ZapLogger).Sync() error` | Flush 缓冲区 |
| `GormLoggerFrom(l, module)` | 创建 GORM 兼容 logger |
| `EntLogFuncFrom(l, module)` | 创建 Ent debug log 函数 |

## 边界约束

- 本包负责日志抽象与桥接，不负责日志采集、存储或观测平台编排
- 不在这里加入业务埋点语义、审计事件定义或 tracing 采样策略
- module 命名约定应保持通用，不为单个服务定制私有格式

## 常见反模式

- 在共享 logger 包里硬编码业务字段名或领域事件名
- 直接暴露底层 zap 给所有调用方并绕过 `For` / `With` 约定
- 把 GORM / Ent 适配与具体 repository 逻辑耦合

## 使用示例

```go
// 创建 logger（bootstrap 层）
zapLogger := logger.New(bc.App)
appLogger := log.With(zapLogger, "service", "iam", "trace_id", tracing.TraceID())

// 业务层创建模块 Helper（最常用写法）
log: logger.For(l, "user/biz/iam")

// 中间件层带 module 的 logger
logger.With(l, "http/server/iam")

// 需要多个字段时
logger.NewHelper(l, logger.WithField("operation", "createConn"))

// 获取底层 zap（供 franz-go kzap 等）
zapLogger.Zap()
```

## module 命名约定

格式：`组件/层/服务`（去掉 `-service` 后缀）

示例：`"user/biz/iam"`、`"redis/data/iam"`、`"grpc/server/iam"`、`"openfga/infra"`

## 测试

```bash
go test ./obs/logging/...
```

## 维护提示

- 若新增桥接器，优先保持 `ZapLogger` 与 `Helper` API 稳定
- 若调整 module 命名习惯，需同步检查现有日志检索与监控面板依赖
