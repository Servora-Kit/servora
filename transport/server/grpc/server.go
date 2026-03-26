package grpc

import (
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/transport/server"
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

func NewServer(opts ...ServerOption) *kgrpc.Server {
	o := &serverOptions{}
	for _, opt := range opts {
		opt(o)
	}

	var serverOpts []kgrpc.ServerOption

	if o.logger != nil {
		serverOpts = append(serverOpts, kgrpc.Logger(o.logger))
	}
	if len(o.middleware) > 0 {
		serverOpts = append(serverOpts, kgrpc.Middleware(o.middleware...))
	}

	if o.conf != nil {
		if o.conf.Network != "" {
			serverOpts = append(serverOpts, kgrpc.Network(o.conf.Network))
		}
		if o.conf.Addr != "" {
			serverOpts = append(serverOpts, kgrpc.Address(o.conf.Addr))
		}
		if o.conf.Timeout != nil {
			serverOpts = append(serverOpts, kgrpc.Timeout(o.conf.Timeout.AsDuration()))
		}
		if o.conf.Tls != nil && o.conf.Tls.Enable {
			tlsCfg := server.MustLoadTLS(o.conf.Tls)
			creds := credentials.NewTLS(tlsCfg)
			serverOpts = append(serverOpts, kgrpc.Options(grpc.Creds(creds)))
		}
	}

	srv := kgrpc.NewServer(serverOpts...)

	for _, reg := range o.registrars {
		reg(srv)
	}

	return srv
}
