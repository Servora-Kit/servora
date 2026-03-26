# AGENTS.md - platform/bootstrap/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-22 | Updated: 2026-03-22 -->

## 模块目的

提供服务启动引导能力，组织配置加载、服务身份解析、业务组件扫描以及 Runtime 生命周期管理。

## 当前结构

```text
platform/bootstrap/
├── bootstrap.go
├── bootstrap_test.go
└── config/
```

## 当前实现事实

- `bootstrap.go` 提供 `SvcIdentity`、`Runtime`、`BootstrapOption`、`WithEnvPrefix`、`ScanBiz`、`BootstrapAndRun` 等启动主线能力
- `Runtime` 负责持有并释放启动过程中创建的资源
- `WithEnvPrefix` 用于控制环境变量前缀装配行为
- `config/loader.go` 承担配置加载职责，但仍属于 bootstrap 启动链的一部分

## 边界约束

- 本包负责“如何把服务拉起来”，不负责承载具体业务初始化逻辑
- 不在这里放业务服务、repository 或 transport handler 的实现
- 不越级侵入 `config/` 子目录的细节说明；子目录需要更细规则时应在其下单独维护文档

## 常见反模式

- 在 bootstrap 中直接编排大量业务逻辑，导致启动层与领域层耦合
- 忽略 `cleanup func()` / Runtime 释放顺序，造成资源泄漏
- 把环境变量命名约定散落到各服务，而不是统一走 bootstrap 选项

## 测试与使用

```bash
go test ./platform/bootstrap/...
```

## 维护提示

- 若新增启动阶段资源，请明确其加入 Runtime 的创建与释放顺序
- 若修改配置装配规则，优先检查所有服务 `configs/` 与部署环境变量是否仍兼容
