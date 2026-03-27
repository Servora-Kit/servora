package client

import (
	"context"
	"fmt"
	"sync"

	"github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/obs/logging"
	grpcclient "github.com/Servora-Kit/servora/transport/client/grpc"
	httpclient "github.com/Servora-Kit/servora/transport/client/http"
	clientmw "github.com/Servora-Kit/servora/transport/client/middleware"
	"github.com/Servora-Kit/servora/transport/runtime"
	klog "github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/registry"
)

type client struct {
	mu        sync.RWMutex
	registry  *runtime.Registry
	buildIn   runtime.ClientBuildInput
	factories map[string]runtime.ClientFactory
}

// NewDefaultClient 使用内建插件和默认配置构建 Client，便于依赖注入场景直接使用。
func NewDefaultClient(
	dataCfg *conf.Data,
	traceCfg *conf.Trace,
	discovery registry.Discovery,
	l logger.Logger,
) (Client, error) {
	return NewClient(dataCfg, traceCfg, discovery, l)
}

func NewClient(
	dataCfg *conf.Data,
	traceCfg *conf.Trace,
	discovery registry.Discovery,
	l logger.Logger,
	opts ...Option,
) (Client, error) {
	o := defaultOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	r := o.registry
	if r == nil {
		r = runtime.NewRegistry()
	}

	if o.registerBuiltin {
		if err := RegisterPlugins(r, &grpcclient.Plugin{}, &httpclient.Plugin{}); err != nil {
			return nil, err
		}
	}
	if len(o.plugins) > 0 {
		if err := RegisterPlugins(r, o.plugins...); err != nil {
			return nil, err
		}
	}

	clientLogger := logger.With(l, "client/transport")
	if clientLogger == nil {
		clientLogger = klog.DefaultLogger
	}
	mw := o.middleware
	if len(mw) == 0 {
		mw = clientmw.NewChainBuilder(clientLogger).
			WithTrace(traceCfg).
			Build()
	}

	buildIn := runtime.ClientBuildInput{
		Data:       dataCfg,
		Trace:      traceCfg,
		Discovery:  discovery,
		Logger:     clientLogger,
		Middleware: mw,
	}

	return &client{
		registry:  r,
		buildIn:   buildIn,
		factories: make(map[string]runtime.ClientFactory, 2),
	}, nil
}

func (c *client) Dial(ctx context.Context, in runtime.ClientDialInput) (runtime.Connection, error) {
	factory, err := c.resolveFactory(in.Protocol)
	if err != nil {
		return nil, err
	}
	conn, err := factory.Dial(ctx, in)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (c *client) resolveFactory(protocol string) (runtime.ClientFactory, error) {
	c.mu.RLock()
	f, ok := c.factories[protocol]
	c.mu.RUnlock()
	if ok {
		return f, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if f, ok = c.factories[protocol]; ok {
		return f, nil
	}

	p, ok := c.registry.Client(protocol)
	if !ok {
		return nil, fmt.Errorf("%w: client type=%s", runtime.ErrPluginNotFound, protocol)
	}
	factory, err := p.Build(context.Background(), c.buildIn)
	if err != nil {
		return nil, fmt.Errorf("build client plugin %q: %w", protocol, err)
	}
	c.factories[protocol] = factory
	return factory, nil
}
