package grpc

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"

	sharedendpoint "github.com/Servora-Kit/servora/transport/shared/endpoint"
	sharedtls "github.com/Servora-Kit/servora/transport/shared/tls"
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
		listen := o.conf.GetListen()
		if network := strings.TrimSpace(listen.GetNetwork()); network != "" {
			serverOpts = append(serverOpts, kgrpc.Network(network))
		}
		bindAddr := strings.TrimSpace(listen.GetAddr())
		if bindAddr != "" {
			serverOpts = append(serverOpts, kgrpc.Address(bindAddr))
		}
		if timeout := listen.GetTimeout(); timeout != nil {
			serverOpts = append(serverOpts, kgrpc.Timeout(timeout.AsDuration()))
		}
		if tlsCfg := sharedtls.MustBuildServerTLS(o.conf.GetTls()); tlsCfg != nil {
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

		endpoint, err := sharedendpoint.ResolveRegistryEndpoint(sharedendpoint.RegistryEndpointInput{
			Scheme:   scheme,
			BindAddr: bindAddr,
			Endpoint: registryEndpoint,
			Host:     registryHost,
			Query:    q,
		})
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
