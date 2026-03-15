# AGENTS.md - pkg/governance/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-15 | Updated: 2026-03-15 -->

## 模块目的

提供服务治理相关基础能力，当前已分成三块：
- `registry/`：注册与发现
- `config/`：配置中心
- `telemetry/`：指标与链路追踪辅助能力

## 当前结构

```text
pkg/governance/
├── config/
├── registry/
└── telemetry/
```

## 当前实现事实

- `registry/` 支持 `consul`、`etcd`、`nacos`、`kubernetes`
- `registry/registry.go` 提供统一入口；目录内还有 `etcd_watcher.go`
- `config/` 目录承载 Consul / Etcd / Nacos 配置源实现
- `telemetry/` 目录当前包含 `metrics.go` 与 `tracing.go`

## 使用位置

- `app/iam/service/internal/server/` 通过 `registry.NewRegistrar` 与 `telemetry.NewMetrics` 接入
- `app/iam/service/internal/data/data.go` 通过 `registry.NewDiscovery` 注入服务发现

## 测试

```bash
go test ./pkg/governance/registry/...
go test ./pkg/governance/config/...
```
