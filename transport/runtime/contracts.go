package runtime

import (
	"context"
	"net/url"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/registry"
)

// Server 定义 transport server 的最小运行时契约。
type Server interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Endpoint() (*url.URL, error)
}

// Connection 定义 transport client 连接的最小运行时契约。
type Connection interface {
	Value() any
	Close() error
	IsHealthy() bool
}

// ClientFactory 定义按服务名构建连接的运行时工厂。
type ClientFactory interface {
	CreateConn(ctx context.Context, serviceName string) (Connection, error)
}

// ServerPlugin 定义 server 协议插件构建接口。
type ServerPlugin interface {
	Type() string
	Build(ctx context.Context, in ServerBuildInput) (Server, error)
}

// ClientPlugin 定义 client 协议插件构建接口。
type ClientPlugin interface {
	Type() string
	Build(ctx context.Context, in ClientBuildInput) (ClientFactory, error)
}

// ServerBuildInput 为 server 插件提供标准输入。
type ServerBuildInput struct {
	Config      any
	Logger      log.Logger
	Middleware  []middleware.Middleware
	Registrars  []any
	ExtraValues map[string]any
}

// ClientBuildInput 为 client 插件提供标准输入。
type ClientBuildInput struct {
	Data        *conf.Data
	Trace       *conf.Trace
	Discovery   registry.Discovery
	Logger      log.Logger
	ExtraValues map[string]any
}
