package grpc

import (
	"fmt"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	"google.golang.org/protobuf/types/known/durationpb"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/transport/server"
	sharedendpoint "github.com/Servora-Kit/servora/transport/shared/endpoint"
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
		listen := o.conf.GetListen()
		network := ""
		addr := ""
		var timeout *durationpb.Duration
		if listen != nil {
			if v := strings.TrimSpace(listen.GetNetwork()); v != "" {
				network = v
			}
			if v := strings.TrimSpace(listen.GetAddr()); v != "" {
				addr = v
			}
			if v := listen.GetTimeout(); v != nil {
				timeout = v
			}
		}
		if network != "" {
			serverOpts = append(serverOpts, kgrpc.Network(network))
		}
		if addr != "" {
			serverOpts = append(serverOpts, kgrpc.Address(addr))
		}
		if timeout != nil {
			serverOpts = append(serverOpts, kgrpc.Timeout(timeout.AsDuration()))
		}
		if o.conf.Tls != nil && o.conf.Tls.Enable {
			tlsCfg := server.MustLoadTLS(o.conf.Tls)
			serverOpts = append(serverOpts, kgrpc.TLSConfig(tlsCfg))
		}

		registryEndpoint := ""
		registryHost := ""
		if reg := o.conf.GetRegistry(); reg != nil {
			registryEndpoint = reg.GetEndpoint()
			registryHost = reg.GetHost()
		}

		endpoint, err := sharedendpoint.ResolveRegistryEndpoint(
			"grpc",
			addr,
			registryEndpoint,
			registryHost,
			o.conf.GetTls() != nil && o.conf.GetTls().GetEnable(),
		)
		if err != nil {
			panic(fmt.Sprintf("resolve grpc registry endpoint: %v", err))
		}
		if endpoint != nil {
			serverOpts = append(serverOpts, kgrpc.Endpoint(endpoint))
		}
	}

	srv := kgrpc.NewServer(serverOpts...)

	for _, reg := range o.registrars {
		reg(srv)
	}

	return srv
}
