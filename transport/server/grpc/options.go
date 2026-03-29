package grpc

import (
	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
)

type Registrar func(*kgrpc.Server)

type ServerOption func(*serverOptions)

type serverOptions struct {
	conf       *conf.Server_GRPC
	logger     log.Logger
	middleware []middleware.Middleware
	registrars []Registrar
}

func WithConfig(c *conf.Server_GRPC) ServerOption {
	return func(o *serverOptions) {
		o.conf = c
	}
}

func WithLogger(l log.Logger) ServerOption {
	return func(o *serverOptions) {
		o.logger = l
	}
}

func WithMiddleware(mw ...middleware.Middleware) ServerOption {
	return func(o *serverOptions) {
		o.middleware = mw
	}
}

func WithServices(registrars ...Registrar) ServerOption {
	return func(o *serverOptions) {
		o.registrars = registrars
	}
}
