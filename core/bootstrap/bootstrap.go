package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
	"github.com/Servora-Kit/servora/core/bootstrap/config"
	slogger "github.com/Servora-Kit/servora/obs/logger"
	"github.com/Servora-Kit/servora/obs/logger/kratosv2"
	"github.com/Servora-Kit/servora/obs/telemetry"

	"github.com/go-kratos/kratos/v2"
	kconfig "github.com/go-kratos/kratos/v2/config"
	kratoslog "github.com/go-kratos/kratos/v2/log"
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
	Bootstrap    *corev1.Bootstrap
	Config       kconfig.Config
	Identity     SvcIdentity
	Logger       *slog.Logger
	KratosLogger kratoslog.Logger

	configCloser func()
	traceCloser  func()
	logCloser    func(context.Context) error
}

// appBuilder 负责基于 Runtime 构造应用并返回清理函数。
type appBuilder func(runtime *Runtime) (app *kratos.App, cleanup func(), err error)

// BootstrapOption 配置启动行为的可选项。
type BootstrapOption func(*bootstrapOptions)

type bootstrapOptions struct {
	envPrefix      bool
	logHandlerFunc slogger.LogHandlerFunc
}

// WithEnvPrefix 启用环境变量前缀。
func WithEnvPrefix() BootstrapOption {
	return func(o *bootstrapOptions) { o.envPrefix = true }
}

// WithLogHandlerFunc 替换默认 zerolog 编码引擎。
// 工厂函数对每个本地后端（stdout/file）调一次，servora 提供 io.Writer。
func WithLogHandlerFunc(f slogger.LogHandlerFunc) BootstrapOption {
	return func(o *bootstrapOptions) { o.logHandlerFunc = f }
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
		bc.App = &corev1.App{}
	}

	hostname, _ := os.Hostname()
	identity := resolveServiceIdentity(name, version, hostname, bc.App)

	var lopts []slogger.Option
	if opts.logHandlerFunc != nil {
		lopts = append(lopts, slogger.WithLogHandlerFunc(opts.logHandlerFunc))
	}
	sl, logCloser := slogger.New(bc, lopts...)
	appLogger := sl.With("service", identity.Name)
	kl := kratosv2.Wrap(appLogger)

	traceCleanup, err := telemetry.InitTracerProvider(bc.Trace, identity.Name, bc.App.Env)
	if err != nil {
		_ = logCloser(context.Background())
		_ = c.Close()
		return nil, err
	}

	return &Runtime{
		Bootstrap:    bc,
		Config:       c,
		Identity:     identity,
		Logger:       appLogger,
		KratosLogger: kl,
		configCloser: func() {
			_ = c.Close()
		},
		traceCloser: traceCleanup,
		logCloser:   logCloser,
	}, nil
}

// Close 释放 Runtime 关联的外部资源。
func (r *Runtime) Close() {
	if r == nil {
		return
	}
	if r.traceCloser != nil {
		r.traceCloser()
	}
	if r.logCloser != nil {
		_ = r.logCloser(context.Background())
	}
	if r.configCloser != nil {
		r.configCloser()
	}
}

// ScanConf 从 Runtime 的合并配置中扫描配置。
// 泛型参数 T 可为业务 conf protobuf message 或任意可反序列化结构体。
func ScanConf[T any](rt *Runtime) (*T, error) {
	if rt == nil || rt.Config == nil {
		return nil, errors.New("runtime config is nil")
	}

	cfg := new(T)
	if err := rt.Config.Scan(cfg); err != nil {
		return nil, fmt.Errorf("scan config: %w", err)
	}
	return cfg, nil
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

func logStage(l *slog.Logger, stage string, keyvals ...any) {
	if l == nil {
		return
	}
	l.Info(stage, keyvals...)
}

// resolveServiceIdentity 解析并回填服务身份默认值。
func resolveServiceIdentity(defaultName, defaultVersion, hostname string, app *corev1.App) SvcIdentity {
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
