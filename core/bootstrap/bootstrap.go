package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
	"github.com/Servora-Kit/servora/core/bootstrap/config"
	slogger "github.com/Servora-Kit/servora/obs/logger"
	"github.com/Servora-Kit/servora/obs/logger/kratosv2"
	"github.com/Servora-Kit/servora/obs/telemetry"

	"github.com/go-kratos/kratos/v2"
	kconfig "github.com/go-kratos/kratos/v2/config"
	kratoslog "github.com/go-kratos/kratos/v2/log"
)

// Runtime 聚合启动阶段产物与 runtime 级资源清理句柄。
type Runtime struct {
	Bootstrap *corev1.Bootstrap
	Config    kconfig.Config
	Logger    *slog.Logger

	serviceID    string
	kratosLogger kratoslog.Logger

	closeOnce sync.Once
	closeErr  error
	cleanups  []func(context.Context) error
}

// Option 配置 Runtime 创建行为。
type Option func(*options)

type options struct {
	name      string
	version   string
	envPrefix bool
}

// Name 设置配置加载前的服务名默认值。
func Name(name string) Option {
	return func(o *options) { o.name = name }
}

// Version 设置配置加载前的服务版本默认值。
func Version(version string) Option {
	return func(o *options) { o.version = version }
}

// WithEnvPrefix 启用基于 Name option 的环境变量前缀。
func WithEnvPrefix() Option {
	return func(o *options) { o.envPrefix = true }
}

var hostnameFn = os.Hostname

// NewRuntime 加载配置并初始化日志、追踪与 Kratos 应用默认项。
func NewRuntime(configPath string, opts ...Option) (*Runtime, error) {
	var o options
	for _, fn := range opts {
		fn(&o)
	}
	if o.envPrefix && o.name == "" {
		return nil, errors.New("bootstrap: WithEnvPrefix requires Name option")
	}

	bc, c, err := config.LoadBootstrap(configPath, o.name, o.envPrefix)
	if err != nil {
		return nil, err
	}

	if bc.App == nil {
		bc.App = &corev1.App{}
	}
	if err := resolveAppIdentity(bc.App, o.name, o.version); err != nil {
		_ = c.Close()
		return nil, err
	}

	hostname, err := hostnameFn()
	if err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("bootstrap: hostname: %w", err)
	}
	serviceID := fmt.Sprintf("%s-%s", bc.App.Name, hostname)

	sl, logCloser := slogger.New(bc)
	appLogger := sl.With("service", bc.App.Name)
	kl := kratosv2.Wrap(appLogger)

	traceCleanup, err := telemetry.InitTracerProvider(bc.Trace, bc.App.Name, bc.App.Env)
	if err != nil {
		_ = logCloser(context.Background())
		_ = c.Close()
		return nil, err
	}

	return &Runtime{
		Bootstrap:    bc,
		Config:       c,
		Logger:       appLogger,
		serviceID:    serviceID,
		kratosLogger: kl,
		cleanups: []func(context.Context) error{
			func(context.Context) error { return c.Close() },
			logCloser,
			func(context.Context) error { traceCleanup(); return nil },
		},
	}, nil
}

// NewApp 使用 Runtime 默认项构造 Kratos 应用。
func (r *Runtime) NewApp(opts ...kratos.Option) *kratos.App {
	appOpts := []kratos.Option{
		kratos.ID(r.serviceID),
		kratos.Name(r.Bootstrap.App.Name),
		kratos.Version(r.Bootstrap.App.Version),
		kratos.Metadata(r.Bootstrap.App.Metadata),
		kratos.Logger(r.kratosLogger),
	}
	appOpts = append(appOpts, opts...)
	return kratos.New(appOpts...)
}

// Run 构建并运行 Kratos 应用，确保业务 cleanup 先于 Runtime 资源关闭。
func (r *Runtime) Run(build func() (*kratos.App, func(), error)) (err error) {
	defer func() {
		err = errors.Join(err, r.Close(context.Background()))
	}()

	app, cleanup, err := build()
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}
	return app.Run()
}

// Close 释放 Runtime 创建的资源。重复调用返回第一次关闭得到的同一个错误。
func (r *Runtime) Close(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.closeOnce.Do(func() {
		var joined error
		for i := len(r.cleanups) - 1; i >= 0; i-- {
			joined = errors.Join(joined, runCleanup(ctx, r.cleanups[i]))
			if i > 0 {
				if err := ctx.Err(); err != nil {
					joined = errors.Join(joined, err)
					break
				}
			}
		}
		r.closeErr = joined
	})
	return r.closeErr
}

func runCleanup(ctx context.Context, cleanup func(context.Context) error) (err error) {
	if cleanup == nil {
		return nil
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = errors.Join(err, fmt.Errorf("panic: %v", recovered))
		}
	}()
	return cleanup(ctx)
}

func resolveAppIdentity(app *corev1.App, defaultName, defaultVersion string) error {
	if app.Name == "" {
		app.Name = defaultName
	}
	if app.Version == "" {
		app.Version = defaultVersion
	}
	if app.Name == "" {
		return errors.New("bootstrap: app name is required")
	}
	if app.Version == "" {
		return errors.New("bootstrap: app version is required")
	}
	if app.Metadata == nil {
		app.Metadata = make(map[string]string)
	}
	return nil
}
