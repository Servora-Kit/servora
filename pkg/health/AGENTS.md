<!-- Parent: ../AGENTS.md -->
# 健康探针 (pkg/health)

**最后更新时间**: 2026-03-08

## 模块目的

提供组件化的 Health/Readiness 探针能力，采用 Builder 模式支持可插拔的 checker 注册。

## 当前文件

- `health.go`：`Checker`、`Pinger` 接口、`Handler`、`PingChecker` 工厂
- `builder.go`：`Builder` 链式构建器，支持 `WithRedisChecker`、`WithChecker`、`WithFuncChecker`
- `defaults.go`：`NewHandlerWithDefaults` 便捷构造，自动注册默认依赖

## 当前实现事实

- Liveness (`/healthz`) 始终返回 200，不执行 checker
- Readiness (`/readyz`) 执行所有注册的 checker，带 3 秒超时
- `PingChecker` 兼容 `redis.Client`、`sql.DB` 等实现 `Ping(ctx) error` 的类型
- 通过 `pkg/transport/server/http.WithHealthCheck()` option 注入 HTTP server

## 使用示例

### 方式 1：使用默认依赖（推荐）
```go
h := health.NewHandlerWithDefaults(health.DefaultDeps{
    Redis: redisClient,
})
srv := http.NewServer(http.WithHealthCheck(h))
```

### 方式 2：使用 Builder 自定义
```go
h := health.NewBuilder().
    WithRedisChecker("redis", redisClient).
    WithFuncChecker("custom", func(ctx context.Context) error {
        return checkSomething(ctx)
    }).
    Build()
srv := http.NewServer(http.WithHealthCheck(h))
```

## 测试

```bash
go test ./pkg/health/...
```
