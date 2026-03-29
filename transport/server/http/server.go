package http

import (
	"fmt"
	"strings"

	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/Servora-Kit/servora/platform/swagger"
	"github.com/Servora-Kit/servora/transport/server"
	svrmw "github.com/Servora-Kit/servora/transport/server/middleware"
	sharedendpoint "github.com/Servora-Kit/servora/transport/shared/endpoint"
)

func NewServer(opts ...ServerOption) *khttp.Server {
	o := &serverOptions{}
	for _, opt := range opts {
		opt(o)
	}

	var serverOpts []khttp.ServerOption

	if o.logger != nil {
		serverOpts = append(serverOpts, khttp.Logger(o.logger))
	}
	if len(o.middleware) > 0 {
		serverOpts = append(serverOpts, khttp.Middleware(o.middleware...))
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
			serverOpts = append(serverOpts, khttp.Network(network))
		}
		if addr != "" {
			serverOpts = append(serverOpts, khttp.Address(addr))
		}
		if timeout != nil {
			serverOpts = append(serverOpts, khttp.Timeout(timeout.AsDuration()))
		}
		if o.conf.Tls != nil && o.conf.Tls.Enable {
			tlsCfg := server.MustLoadTLS(o.conf.Tls)
			serverOpts = append(serverOpts, khttp.TLSConfig(tlsCfg))
		}

		registryEndpoint := ""
		registryHost := ""
		if reg := o.conf.GetRegistry(); reg != nil {
			registryEndpoint = reg.GetEndpoint()
			registryHost = reg.GetHost()
		}

		endpoint, err := sharedendpoint.ResolveRegistryEndpoint(
			"http",
			addr,
			registryEndpoint,
			registryHost,
			o.conf.GetTls() != nil && o.conf.GetTls().GetEnable(),
		)
		if err != nil {
			panic(fmt.Sprintf("resolve http registry endpoint: %v", err))
		}
		if endpoint != nil {
			serverOpts = append(serverOpts, khttp.Endpoint(endpoint))
		}
	}

	if svrmw.IsEnabled(o.cors) {
		serverOpts = append(serverOpts, khttp.Filter(svrmw.Middleware(o.cors)))
	}

	srv := khttp.NewServer(serverOpts...)

	if o.metricsHandler != nil {
		srv.Handle("/metrics", o.metricsHandler)
	}

	if o.healthHandler != nil {
		srv.HandleFunc("/healthz", o.healthHandler.LivenessHandler())
		srv.HandleFunc("/readyz", o.healthHandler.ReadinessHandler())
	}

	if len(o.swaggerSpec) > 0 {
		swagger.Register(srv, o.swaggerSpec, o.swaggerOpts...)
	}

	for _, reg := range o.registrars {
		reg(srv)
	}

	return srv
}
