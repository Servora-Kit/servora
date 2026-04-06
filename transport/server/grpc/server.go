package grpc

import (
	"fmt"
	"net/url"
	"strconv"

	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"

	"github.com/Servora-Kit/servora/transport/server"
	sharedconfig "github.com/Servora-Kit/servora/transport/shared/config"
	sharedendpoint "github.com/Servora-Kit/servora/transport/shared/endpoint"
)

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
		lc := sharedconfig.ParseListenConfig(o.conf.GetListen())
		if lc.Network != "" {
			serverOpts = append(serverOpts, kgrpc.Network(lc.Network))
		}
		if lc.Addr != "" {
			serverOpts = append(serverOpts, kgrpc.Address(lc.Addr))
		}
		if lc.Timeout != nil {
			serverOpts = append(serverOpts, kgrpc.Timeout(lc.Timeout.AsDuration()))
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

		secure := o.conf.GetTls() != nil && o.conf.GetTls().GetEnable()
		scheme := "grpc"
		if secure {
			scheme = "grpcs"
		}
		q := url.Values{}
		q.Set("isSecure", strconv.FormatBool(secure))

		endpoint, err := sharedendpoint.ResolveRegistryEndpoint(
			scheme,
			lc.Addr,
			registryEndpoint,
			registryHost,
			q,
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
