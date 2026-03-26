package client

import (
	"context"
	"fmt"
	"sync"

	"github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/obs/logging"
	grpcclient "github.com/Servora-Kit/servora/transport/client/grpc"
	httpclient "github.com/Servora-Kit/servora/transport/client/http"
	"github.com/Servora-Kit/servora/transport/runtime"
	"github.com/go-kratos/kratos/v2/registry"
)

type client struct {
	mu        sync.RWMutex
	registry  *runtime.Registry
	buildIn   runtime.ClientBuildInput
	factories map[ConnType]runtime.ClientFactory
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

	buildIn := runtime.ClientBuildInput{
		Data:      dataCfg,
		Trace:     traceCfg,
		Discovery: discovery,
		Logger:    logger.With(l, "client/transport"),
	}

	return &client{
		registry:  r,
		buildIn:   buildIn,
		factories: make(map[ConnType]runtime.ClientFactory, 2),
	}, nil
}

func (c *client) CreateConn(ctx context.Context, connType ConnType, serviceName string) (Connection, error) {
	factory, err := c.resolveFactory(connType)
	if err != nil {
		return nil, err
	}
	conn, err := factory.CreateConn(ctx, serviceName)
	if err != nil {
		return nil, err
	}
	return runtimeConnAdapter{
		conn:     conn,
		connType: connType,
	}, nil
}

func (c *client) resolveFactory(connType ConnType) (runtime.ClientFactory, error) {
	c.mu.RLock()
	f, ok := c.factories[connType]
	c.mu.RUnlock()
	if ok {
		return f, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if f, ok = c.factories[connType]; ok {
		return f, nil
	}

	p, ok := c.registry.Client(string(connType))
	if !ok {
		return nil, fmt.Errorf("%w: client type=%s", runtime.ErrPluginNotFound, connType)
	}
	factory, err := p.Build(context.Background(), c.buildIn)
	if err != nil {
		return nil, fmt.Errorf("build client plugin %q: %w", connType, err)
	}
	c.factories[connType] = factory
	return factory, nil
}

type runtimeConnAdapter struct {
	conn     runtime.Connection
	connType ConnType
}

func (a runtimeConnAdapter) Value() any      { return a.conn.Value() }
func (a runtimeConnAdapter) Close() error    { return a.conn.Close() }
func (a runtimeConnAdapter) IsHealthy() bool { return a.conn.IsHealthy() }
func (a runtimeConnAdapter) GetType() ConnType {
	return a.connType
}
