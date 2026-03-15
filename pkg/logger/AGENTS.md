# AGENTS.md - pkg/logger/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-15 | Updated: 2026-03-15 -->

## 模块目的

基于 Zap 适配 Kratos 日志接口，并提供 GORM / Ent 日志桥接与结构化字段辅助能力。

## 当前文件

- `log.go`：主日志实现、`Config`、`WithModule`、`WithField`
- `gorm_log.go`：GORM 日志适配
- `ent_log.go`：Ent 调试日志桥接

## 当前实现事实

- `NewLogger` 会在配置存在且 `Filename` 为空时默认回落到 `./logs/app.log`
- 支持 `dev`、`prod`、`test` 三类环境输出策略
- `WithModule` 约定 module 命名形如 `组件/层/服务名`
- `NewHelper` 支持额外 `Option` 注入

## 使用示例

```go
l := logger.NewLogger(&logger.Config{Env: "dev"})
helper := logger.NewHelper(l, logger.WithModule("auth/biz/iam-service"))
helper.Info("service started")
```

## 测试

```bash
go test ./pkg/logger/...
```
