package bootstrap

import (
	"errors"
	"fmt"
	"os"
	"time"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/platform/bootstrap/config"
	"github.com/Servora-Kit/servora/obs/telemetry"
	"github.com/Servora-Kit/servora/obs/logging"

	"github.com/go-kratos/kratos/v2"
	kconfig "github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
)

// SvcIdentity 定义服务实例身份信息。
type SvcIdentity struct {
	Name     string
	Version  string
	ID       string
	Metadata map[string]string
}

// Runtime 聚合启动阶段产物与资源清理句柄。
type Runtime struct {
	Bootstrap *conf.Bootstrap
	Config    kconfig.Config
	Identity  SvcIdentity
	Logger    log.Logger

	configCloser func()
	traceCloser  func()
}

// appBuilder 负责基于 Runtime 构造应用并返回清理函数。
type appBuilder func(runtime *Runtime) (app *kratos.App, cleanup func(), err error)

// BootstrapOption 配置启动行为的可选项。
type BootstrapOption func(*bootstrapOptions)

type bootstrapOptions struct {
	envPrefix bool
}

// WithEnvPrefix 启用环境变量前缀。
// 启用后，配置加载器会根据服务名推导前缀（如 iam.service → IAM_），
// 仅读取带前缀的环境变量覆盖配置。
// 默认不使用前缀，直接读取无前缀的环境变量。
func WithEnvPrefix() BootstrapOption {
	return func(o *bootstrapOptions) { o.envPrefix = true }
}

// runtimeFactory 负责创建 Runtime。
type runtimeFactory func(configPath, name, version string, opts bootstrapOptions) (*Runtime, error)

// appRunner 负责运行应用主循环。
type appRunner func(app *kratos.App) error

// runner 封装启动链路中的可替换依赖。
type runner struct {
	newRuntime runtimeFactory
	runApp     appRunner
}

var (
	// 通过默认 runner 注入依赖，便于单测替换而不污染全局状态。
	defaultRunner = newRunner(newRuntime, run)
)

// newRunner 创建 runner，空依赖会回退到默认实现。
func newRunner(runtimeFactory runtimeFactory, appRunner appRunner) runner {
	if runtimeFactory == nil {
		runtimeFactory = newRuntime
	}
	if appRunner == nil {
		appRunner = run
	}
	return runner{newRuntime: runtimeFactory, runApp: appRunner}
}

// newRuntime 加载配置并初始化日志、追踪与身份信息。
func newRuntime(configPath, name, version string, opts bootstrapOptions) (*Runtime, error) {
	bc, c, err := config.LoadBootstrap(configPath, name, opts.envPrefix)
	if err != nil {
		return nil, err
	}

	if bc.App == nil {
		bc.App = &conf.App{}
	}

	hostname, _ := os.Hostname()
	identity := resolveServiceIdentity(name, version, hostname, bc.App)

	zapLogger := logger.New(bc.App)
	appLogger := log.With(
		zapLogger,
		"service", identity.Name,
		"trace_id", tracing.TraceID(),
		"span_id", tracing.SpanID(),
	)

	traceCleanup, err := telemetry.InitTracerProvider(bc.Trace, identity.Name, bc.App.Env)
	if err != nil {
		_ = c.Close()
		return nil, err
	}

	return &Runtime{
		Bootstrap: bc,
		Config:    c,
		Identity:  identity,
		Logger:    appLogger,
		configCloser: func() {
			_ = c.Close()
		},
		traceCloser: traceCleanup,
	}, nil
}

// Close 释放 Runtime 关联的外部资源。
func (r *Runtime) Close() {
	if r == nil {
		return
	}
	// 先关闭 tracer，确保 trace 在底层资源关闭前完成 flush。
	if r.traceCloser != nil {
		r.traceCloser()
	}
	if r.configCloser != nil {
		r.configCloser()
	}
}

// ScanBiz 从 Runtime 的合并配置中扫描服务私有业务配置。
// 泛型参数 B 通常为各服务 conf 包中的 Biz protobuf message 类型。
func ScanBiz[B any](rt *Runtime) (*B, error) {
	biz := new(B)
	if err := rt.Config.Scan(biz); err != nil {
		return nil, fmt.Errorf("scan biz config: %w", err)
	}
	return biz, nil
}

// run 执行 kratos 应用。
func run(app *kratos.App) error {
	return app.Run()
}

// runWithRuntime 在已构造 Runtime 的前提下装配并运行应用。
func (r runner) runWithRuntime(runtime *Runtime, builder appBuilder) error {
	if runtime == nil {
		return errors.New("runtime is nil")
	}

	logStage(runtime.Logger, "run_with_runtime_start", "service", runtime.Identity.Name, "version", runtime.Identity.Version)
	startedAt := time.Now()
	if builder == nil {
		return errors.New("app builder is nil")
	}

	app, cleanup, err := builder(runtime)
	if err != nil {
		logStage(runtime.Logger, "run_with_runtime_failed", "reason", "build_app", "error", err.Error())
		return err
	}
	if app == nil {
		// app 为空说明启动装配链路异常，直接失败避免后续 panic。
		logStage(runtime.Logger, "run_with_runtime_failed", "reason", "nil_app")
		return errors.New("app is nil")
	}
	if cleanup != nil {
		defer cleanup()
	}

	err = r.runApp(app)
	if err != nil {
		logStage(runtime.Logger, "run_with_runtime_failed", "reason", "run_app", "error", err.Error())
		return err
	}

	logStage(runtime.Logger, "run_with_runtime_done", "duration", time.Since(startedAt).String())
	return nil
}

// BootstrapAndRun 对外暴露统一启动入口。
func BootstrapAndRun(configPath, name, version string, builder appBuilder, opts ...BootstrapOption) error {
	return defaultRunner.bootstrapAndRun(configPath, name, version, builder, opts...)
}

// bootstrapAndRun 执行完整启动链路：构造 Runtime、运行应用、回收资源。
func (r runner) bootstrapAndRun(configPath, name, version string, builder appBuilder, opts ...BootstrapOption) error {
	var o bootstrapOptions
	for _, fn := range opts {
		fn(&o)
	}
	runtime, err := r.newRuntime(configPath, name, version, o)
	if err != nil {
		return err
	}
	defer runtime.Close()
	logStage(runtime.Logger, "bootstrap_start", "service", runtime.Identity.Name, "version", runtime.Identity.Version)
	startedAt := time.Now()

	err = r.runWithRuntime(runtime, builder)
	if err != nil {
		logStage(runtime.Logger, "bootstrap_failed", "error", err.Error())
		return err
	}

	logStage(runtime.Logger, "bootstrap_done", "duration", time.Since(startedAt).String())
	return nil
}

func logStage(l log.Logger, stage string, keyvals ...any) {
	if l == nil {
		return
	}
	fields := []any{"stage", stage}
	if len(keyvals) > 0 {
		fields = append(fields, keyvals...)
	}
	_ = l.Log(log.LevelInfo, fields...)
}

// resolveServiceIdentity 解析并回填服务身份默认值。
func resolveServiceIdentity(defaultName, defaultVersion, hostname string, app *conf.App) SvcIdentity {
	name := defaultName
	version := defaultVersion
	metadata := make(map[string]string)

	if app != nil {
		// 将默认身份信息回填到 app，保证下游 provider 读取到一致值。
		if app.Name != "" {
			name = app.Name
		} else {
			app.Name = name
		}
		if app.Version != "" {
			version = app.Version
		} else {
			app.Version = version
		}
		if app.Metadata == nil {
			app.Metadata = metadata
		} else {
			metadata = app.Metadata
		}
	}

	id := fmt.Sprintf("%s-%s", name, hostname)
	return SvcIdentity{
		Name:     name,
		Version:  version,
		ID:       id,
		Metadata: metadata,
	}
}
