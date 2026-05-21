# AGENTS.md - core/bootstrap/

<!-- Parent: ../AGENTS.md -->
<!-- Updated: 2026-05-21 -->

## 模块目的

`core/bootstrap` 负责服务启动引导：加载 Bootstrap proto 配置、初始化 runtime 级资源、解析服务身份、扫描配置 section，并按生命周期顺序启动/清理应用。

它只处理“框架如何把服务拉起来”，不承载业务 service、repository、handler 或领域初始化逻辑。

## 当前结构

```text
core/bootstrap/
├── bootstrap.go      # Runtime、SvcIdentity、BootstrapAndRun、ScanConf
├── scan.go           # Section 扫描契约与 ScanSections
├── *_test.go
└── config/           # 配置 loader，仍属于启动链
```

## 公开边界

```go
type Runtime struct { ... }
type SvcIdentity struct { ... }

func BootstrapAndRun(ctx context.Context, fn func(context.Context, *Runtime) error, opts ...BootstrapOption) error
func ScanConf[T any](rt *Runtime, key string, out T) error
func ScanSections(rt *Runtime, sections ...Section) error
func WithEnvPrefix(prefix string) BootstrapOption
func WithLogHandlerFunc(fn obslog.HandlerFunc) BootstrapOption

type Section interface { Key() string }
type OptionalSection interface { Optional() bool }
type Defaulter interface { SetDefaults() }
type RequiredChecker interface { CheckRequired() error }
type ConfApplier interface { ApplyConf(*Runtime) error }
```

旧版业务扫描入口已不存在。遇到历史引用时，按 `ScanConf`/`ScanSections` 当前 API 迁移。

## 生命周期语义

- `newRuntime` 负责加载 config、初始化 logger/Kratos logger/tracer、解析 identity，并登记 closers。
- cleanup 顺序遵循后进先出语义；新增启动期资源时必须明确 close 顺序。
- `ScanSections` 要求 runtime/config/section 非空、section key 非空。
- Optional section 缺失时跳过；非 optional 缺失或解码失败返回错误。
- 扫描后若 section 实现 `ApplyConf(*Runtime)`，立即调用以完成 runtime 级装配。

## 边界约束

- `WithEnvPrefix` 是环境变量前缀入口；不要在业务服务里复制命名约定。
- `config/loader.go` 是 bootstrap 启动链的一部分，但不要把 loader 细节散落到上层文档。
- 不在本包引入具体 transport server、database client、业务 repository 或 handler。
- 修改 Bootstrap proto schema 后同步检查 `api/protos/servora/core/v1/bootstrap.proto`、`make gen` 输出和本包测试。

## 常见反模式

- 在 bootstrap 回调外直接编排大量业务逻辑，导致启动层与领域层耦合。
- 忽略 cleanup，造成 logger/tracer/config watcher 泄漏。
- 让 section 的 `ApplyConf` 做长耗时业务启动；它应只做配置装配。

## 测试

```bash
go test ./core/bootstrap/...
```

重点覆盖：config loader、Runtime cleanup、`ScanConf`、`ScanSections` 必填/可选/default/check/apply 语义。
